package persistence

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrWeakUserPassword       = errors.New("access: password must be 8-16 characters and include an uppercase letter and a digit")
	ErrCurrentPasswordInvalid = errors.New("access: current password invalid")
	ErrPasswordMismatch       = errors.New("access: password confirmation mismatch")
)

func ValidateUserChosenPassword(password string) error {
	if len(password) < 8 || len(password) > 16 {
		return ErrWeakUserPassword
	}
	var upper, digit bool
	for _, r := range password {
		if unicode.IsUpper(r) {
			upper = true
		}
		if unicode.IsDigit(r) {
			digit = true
		}
	}
	if !upper || !digit {
		return ErrWeakUserPassword
	}
	return nil
}

func (store *Store) VerifyUserPassword(ctx context.Context, userID, password string) error {
	if store == nil || store.database == nil || strings.TrimSpace(userID) == "" || password == "" {
		consumeDummyPasswordWork(password)
		return ErrCurrentPasswordInvalid
	}
	if err := verifyUserPasswordByID(ctx, store.database, userID, password); err != nil {
		if errors.Is(err, ErrInvalidUserCredentials) {
			return ErrCurrentPasswordInvalid
		}
		return err
	}
	return nil
}

func (store *Store) ChangeOwnPassword(ctx context.Context, userID, currentPassword, newPassword, confirmation string) error {
	userID = strings.TrimSpace(userID)
	if store == nil || store.database == nil || userID == "" {
		return ErrInvalidUserCredentials
	}
	if newPassword != confirmation {
		return ErrPasswordMismatch
	}
	if err := ValidateUserChosenPassword(newPassword); err != nil {
		return err
	}
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyUserPasswordByID(ctx, tx, userID, currentPassword); err != nil {
			if errors.Is(err, ErrInvalidUserCredentials) {
				return ErrCurrentPasswordInvalid
			}
			return err
		}
		if err := setUserPassword(ctx, tx, userID, newPassword); err != nil {
			return err
		}
		return revokeWebSessionsForUser(ctx, tx, userID)
	})
}

func (store *Store) RecoverPasswordWithCode(
	ctx context.Context,
	protection *VerificationProtection,
	request domain.VerifyChallengeRequest,
	authorizationTTL time.Duration,
	newPassword, confirmation string,
) (domain.AuthorizationConsumptionReceipt, error) {
	if store == nil || store.database == nil || protection == nil || authorizationTTL <= 0 || strings.TrimSpace(request.Code) == "" {
		return domain.AuthorizationConsumptionReceipt{}, domain.ErrVerificationInvalid
	}
	if request.Purpose != domain.VerificationPurposePasswordRecovery {
		return domain.AuthorizationConsumptionReceipt{}, domain.ErrVerificationInvalid
	}
	if newPassword != confirmation {
		return domain.AuthorizationConsumptionReceipt{}, ErrPasswordMismatch
	}
	if err := ValidateUserChosenPassword(newPassword); err != nil {
		return domain.AuthorizationConsumptionReceipt{}, err
	}
	destinationHash, _, err := protection.DestinationHash(request.Channel, request.Destination)
	if err != nil {
		return domain.AuthorizationConsumptionReceipt{}, err
	}
	expectedBinding := protection.BindingHash(request.Purpose, request.UserID, request.TenantID, request.FlowID, request.Channel, destinationHash)
	var receipt domain.AuthorizationConsumptionReceipt
	var outcome error
	err = store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		var challenge verificationChallengeRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("challenge_id = ?", strings.TrimSpace(request.ChallengeID)).First(&challenge).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				outcome = domain.ErrVerificationInvalid
				return nil
			}
			return err
		}
		if !constantVerificationEqual(challenge.BindingHash, expectedBinding) {
			outcome = domain.ErrVerificationInvalid
			return nil
		}
		if challenge.ConsumedAt != nil {
			outcome = domain.ErrVerificationConsumed
			return nil
		}
		if !challenge.ExpiresAt.After(now) {
			outcome = domain.ErrVerificationExpired
			return nil
		}
		if challenge.Attempts >= challenge.MaxAttempts {
			outcome = domain.RateLimitError{Reason: "attempt_limit"}
			return nil
		}
		if !constantVerificationEqual(challenge.CodeHash, protection.HashCode(challenge.ChallengeID, request.Code)) {
			challenge.Attempts++
			if err := tx.Model(&verificationChallengeRecord{}).Where("challenge_id = ?", challenge.ChallengeID).Update("attempts", challenge.Attempts).Error; err != nil {
				return err
			}
			if challenge.Attempts >= challenge.MaxAttempts {
				outcome = domain.RateLimitError{Reason: "attempt_limit"}
			} else {
				outcome = domain.ErrVerificationInvalid
			}
			return nil
		}
		rawAuthorization, err := randomVerificationSecret(32)
		if err != nil {
			return err
		}
		consumedAt := now
		authorization := oneTimeAuthorizationRecord{
			AuthorizationHash: protection.HashAuthorization(rawAuthorization),
			ChallengeID: challenge.ChallengeID,
			BindingHash: challenge.BindingHash,
			Purpose: challenge.Purpose,
			UserID: challenge.UserID,
			TenantID: challenge.TenantID,
			FlowID: challenge.FlowID,
			Channel: challenge.Channel,
			DestinationHash: challenge.DestinationHash,
			ExpiresAt: canonicalVerificationTime(now.Add(authorizationTTL)),
			ConsumedAt: &consumedAt,
			CreatedAt: canonicalVerificationTime(now),
		}
		if err := tx.Create(&authorization).Error; err != nil {
			return err
		}
		if err := setUserPassword(ctx, tx, request.UserID, newPassword); err != nil {
			return err
		}
		if err := revokeWebSessionsForUser(ctx, tx, request.UserID); err != nil {
			return err
		}
		result := tx.Model(&verificationChallengeRecord{}).
			Where("challenge_id = ? AND consumed_at IS NULL", challenge.ChallengeID).
			Update("consumed_at", consumedAt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return domain.ErrVerificationConsumed
		}
		if err := tx.Model(&securityNotificationOutboxRecord{}).
			Where("challenge_id = ? AND state IN ?", challenge.ChallengeID, []string{domain.NotificationStatePending, domain.NotificationStateFailed}).
			Updates(map[string]any{
				"state": domain.NotificationStateCancelled,
				"destination_ciphertext": "",
				"secret_ciphertext": "",
				"failure_code": "RECOVERY_CONSUMED",
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		receipt = domain.AuthorizationConsumptionReceipt{ChallengeID: challenge.ChallengeID, ConsumedAt: consumedAt}
		return nil
	})
	if err != nil {
		return domain.AuthorizationConsumptionReceipt{}, err
	}
	if outcome != nil {
		return domain.AuthorizationConsumptionReceipt{}, outcome
	}
	return receipt, nil
}

func (store *Store) ResetPasswordWithAuthorization(
	ctx context.Context,
	protection *VerificationProtection,
	request domain.ConsumeAuthorizationRequest,
	newPassword, confirmation string,
) (domain.AuthorizationConsumptionReceipt, error) {
	if store == nil || store.database == nil || protection == nil || strings.TrimSpace(request.Code) == "" {
		return domain.AuthorizationConsumptionReceipt{}, domain.ErrVerificationInvalid
	}
	if newPassword != confirmation {
		return domain.AuthorizationConsumptionReceipt{}, ErrPasswordMismatch
	}
	if err := ValidateUserChosenPassword(newPassword); err != nil {
		return domain.AuthorizationConsumptionReceipt{}, err
	}
	destinationHash, _, err := protection.DestinationHash(request.Channel, request.Destination)
	if err != nil {
		return domain.AuthorizationConsumptionReceipt{}, err
	}
	expectedBinding := protection.BindingHash(request.Purpose, request.UserID, request.TenantID, request.FlowID, request.Channel, destinationHash)
	hash := protection.HashAuthorization(request.Code)
	var receipt domain.AuthorizationConsumptionReceipt
	var outcome error
	err = store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		var authorization oneTimeAuthorizationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("authorization_hash = ?", hash).First(&authorization).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				outcome = domain.ErrVerificationInvalid
				return nil
			}
			return err
		}
		if !constantVerificationEqual(authorization.BindingHash, expectedBinding) {
			outcome = domain.ErrVerificationInvalid
			return nil
		}
		if authorization.ConsumedAt != nil {
			outcome = domain.ErrVerificationConsumed
			return nil
		}
		if !authorization.ExpiresAt.After(now) {
			outcome = domain.ErrVerificationExpired
			return nil
		}
		if err := setUserPassword(ctx, tx, request.UserID, newPassword); err != nil {
			return err
		}
		if err := revokeWebSessionsForUser(ctx, tx, request.UserID); err != nil {
			return err
		}
		consumedAt := now
		result := tx.Model(&oneTimeAuthorizationRecord{}).
			Where("authorization_hash = ? AND consumed_at IS NULL", hash).
			Update("consumed_at", consumedAt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			outcome = domain.ErrVerificationConsumed
			return nil
		}
		receipt = domain.AuthorizationConsumptionReceipt{ChallengeID: authorization.ChallengeID, ConsumedAt: consumedAt}
		return nil
	})
	if err != nil {
		return domain.AuthorizationConsumptionReceipt{}, err
	}
	if outcome != nil {
		return domain.AuthorizationConsumptionReceipt{}, outcome
	}
	return receipt, nil
}

func verifyUserPasswordByID(ctx context.Context, database *gorm.DB, userID, password string) error {
	userID = strings.TrimSpace(userID)
	if database == nil || userID == "" || password == "" {
		consumeDummyPasswordWork(password)
		return ErrInvalidUserCredentials
	}
	var credential userPasswordCredentialRecord
	if err := database.WithContext(ctx).Where("user_id = ? AND disabled = ?", userID, false).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			consumeDummyPasswordWork(password)
			return ErrInvalidUserCredentials
		}
		return err
	}
	if credential.Iterations < 100_000 || credential.Iterations > 2_000_000 {
		return ErrInvalidUserCredentials
	}
	salt, err := base64.RawStdEncoding.DecodeString(credential.Salt)
	if err != nil || len(salt) < 16 {
		return ErrInvalidUserCredentials
	}
	expected, err := base64.RawStdEncoding.DecodeString(credential.PasswordHash)
	if err != nil || len(expected) != 32 {
		return ErrInvalidUserCredentials
	}
	actual := pbkdf2SHA256([]byte(password), salt, credential.Iterations, len(expected))
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return ErrInvalidUserCredentials
	}
	return nil
}
