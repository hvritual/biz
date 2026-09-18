package persistence

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type verificationChallengeRecord struct {
	ChallengeID       string     `gorm:"column:challenge_id;primaryKey;size:96"`
	BusinessEventID   string     `gorm:"column:business_event_id;size:160;not null;uniqueIndex"`
	BindingHash       string     `gorm:"column:binding_hash;size:64;not null"`
	Purpose           string     `gorm:"column:purpose;size:64;not null;index:idx_verification_target_time,priority:2"`
	UserID            string     `gorm:"column:user_id;size:64;not null;index:idx_verification_target_time,priority:1"`
	TenantID          string     `gorm:"column:tenant_id;size:64;not null;default:''"`
	FlowID            string     `gorm:"column:flow_id;size:160;not null;index"`
	Channel           string     `gorm:"column:channel;size:16;not null"`
	DestinationHash   string     `gorm:"column:destination_hash;size:64;not null;index:idx_verification_target_time,priority:3"`
	MaskedDestination string     `gorm:"column:masked_destination;size:320;not null"`
	CodeHash          string     `gorm:"column:code_hash;size:64;not null"`
	Attempts          uint32     `gorm:"column:attempts;not null;default:0"`
	MaxAttempts       uint32     `gorm:"column:max_attempts;not null"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;not null;index"`
	ConsumedAt        *time.Time `gorm:"column:consumed_at;index"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null;index:idx_verification_target_time,priority:4"`
}

func (verificationChallengeRecord) TableName() string { return "biz_verification_challenges" }

type oneTimeAuthorizationRecord struct {
	AuthorizationHash string     `gorm:"column:authorization_hash;primaryKey;size:64"`
	ChallengeID       string     `gorm:"column:challenge_id;size:96;not null;uniqueIndex"`
	BindingHash       string     `gorm:"column:binding_hash;size:64;not null"`
	Purpose           string     `gorm:"column:purpose;size:64;not null"`
	UserID            string     `gorm:"column:user_id;size:64;not null;index"`
	TenantID          string     `gorm:"column:tenant_id;size:64;not null;default:''"`
	FlowID            string     `gorm:"column:flow_id;size:160;not null;index"`
	Channel           string     `gorm:"column:channel;size:16;not null"`
	DestinationHash   string     `gorm:"column:destination_hash;size:64;not null"`
	ExpiresAt         time.Time  `gorm:"column:expires_at;not null;index"`
	ConsumedAt        *time.Time `gorm:"column:consumed_at;index"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null"`
}

func (oneTimeAuthorizationRecord) TableName() string { return "biz_one_time_authorizations" }

type securityNotificationOutboxRecord struct {
	EventID               string     `gorm:"column:event_id;primaryKey;size:64"`
	BusinessEventID       string     `gorm:"column:business_event_id;size:160;not null;uniqueIndex"`
	ChallengeID           string     `gorm:"column:challenge_id;size:96;not null;default:'';index"`
	BindingHash           string     `gorm:"column:binding_hash;size:64;not null"`
	Kind                  string     `gorm:"column:kind;size:64;not null"`
	Purpose               string     `gorm:"column:purpose;size:64;not null"`
	UserID                string     `gorm:"column:user_id;size:64;not null;index"`
	TenantID              string     `gorm:"column:tenant_id;size:64;not null;default:''"`
	FlowID                string     `gorm:"column:flow_id;size:160;not null;default:''"`
	Channel               string     `gorm:"column:channel;size:16;not null"`
	DestinationHash       string     `gorm:"column:destination_hash;size:64;not null"`
	MaskedDestination     string     `gorm:"column:masked_destination;size:320;not null"`
	DestinationCiphertext string     `gorm:"column:destination_ciphertext;type:text"`
	SecretCiphertext      string     `gorm:"column:secret_ciphertext;type:text"`
	KeyVersion            string     `gorm:"column:key_version;size:64;not null"`
	State                 string     `gorm:"column:state;size:24;not null;index"`
	Attempts              uint32     `gorm:"column:attempts;not null;default:0"`
	ProviderReceipt       string     `gorm:"column:provider_receipt;size:200;not null;default:''"`
	FailureCode           string     `gorm:"column:failure_code;size:64;not null;default:''"`
	ExpiresAt             time.Time  `gorm:"column:expires_at;not null;index"`
	DeliveredAt           *time.Time `gorm:"column:delivered_at"`
	CreatedAt             time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;not null"`
}

func (securityNotificationOutboxRecord) TableName() string {
	return "biz_security_notification_outbox"
}

type VerificationRepository struct {
	database   *gorm.DB
	protection *VerificationProtection
}

var _ ports.VerificationRepository = (*VerificationRepository)(nil)

func NewVerificationRepository(database *gorm.DB, protection *VerificationProtection) (*VerificationRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: verification database is required")
	}
	if protection == nil {
		return nil, ErrVerificationKeyUnavailable
	}
	return &VerificationRepository{database: database, protection: protection}, nil
}

func (repository *VerificationRepository) EnsureSchema(ctx context.Context) error {
	if repository == nil || repository.database == nil {
		return errors.New("access persistence: verification repository unavailable")
	}
	return repository.database.WithContext(ctx).AutoMigrate(
		&verificationChallengeRecord{},
		&oneTimeAuthorizationRecord{},
		&securityNotificationOutboxRecord{},
	)
}

func (repository *VerificationRepository) CreateVerificationChallenge(ctx context.Context, request domain.VerificationChallengeRequest, policy domain.VerificationPolicy) (domain.VerificationChallengeReceipt, error) {
	if repository == nil || repository.database == nil || repository.protection == nil {
		return domain.VerificationChallengeReceipt{}, domain.ErrVerificationInvalid
	}
	if err := request.Validate(); err != nil {
		return domain.VerificationChallengeReceipt{}, err
	}
	if err := policy.Validate(); err != nil {
		return domain.VerificationChallengeReceipt{}, err
	}
	destinationHash, normalizedDestination, err := repository.protection.DestinationHash(request.Channel, request.Destination)
	if err != nil {
		return domain.VerificationChallengeReceipt{}, err
	}
	bindingHash := repository.protection.BindingHash(request.Purpose, request.UserID, request.TenantID, request.FlowID, request.Channel, destinationHash)
	masked := repository.protection.MaskDestination(request.Channel, normalizedDestination)
	var receipt domain.VerificationChallengeReceipt
	err = repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		existing, found, err := findChallengeByBusinessEvent(ctx, tx, request.BusinessEventID, true)
		if err != nil {
			return err
		}
		if found {
			expectedBinding := repository.notificationBinding(existing, request.Secret)
			if !constantVerificationEqual(existing.BindingHash, expectedBinding) {
				return domain.ErrVerificationConflict
			}
			receipt, err = challengeReceipt(ctx, tx, existing)
			return err
		}

		var latest verificationChallengeRecord
		latestErr := tx.WithContext(ctx).
			Where("user_id = ? AND purpose = ? AND destination_hash = ?", strings.TrimSpace(request.UserID), string(request.Purpose), destinationHash).
			Order("created_at DESC").Limit(1).Take(&latest).Error
		if latestErr != nil && !errors.Is(latestErr, gorm.ErrRecordNotFound) {
			return latestErr
		}
		if latestErr == nil {
			nextAllowed := latest.CreatedAt.Add(policy.ResendInterval)
			if nextAllowed.After(now) {
				return domain.RateLimitError{RetryAfter: nextAllowed.Sub(now), Reason: "resend_interval"}
			}
		}
		var sends int64
		if err := tx.WithContext(ctx).Model(&verificationChallengeRecord{}).
			Where("user_id = ? AND purpose = ? AND destination_hash = ? AND created_at >= ?", strings.TrimSpace(request.UserID), string(request.Purpose), destinationHash, now.Add(-policy.SendLimitWindow)).
			Count(&sends).Error; err != nil {
			return err
		}
		if sends >= int64(policy.MaxSendsPerWindow) {
			return domain.RateLimitError{RetryAfter: policy.SendLimitWindow, Reason: "send_window_limit"}
		}

		challengeSecret, err := randomVerificationSecret(24)
		if err != nil {
			return err
		}
		challengeID := "vch-" + challengeSecret
		code, err := repository.protection.GenerateCode(policy.CodeDigits)
		if err != nil {
			return err
		}
		eventID := stableSecurityEventID(request.BusinessEventID)
		destinationCiphertext, keyVersion, err := repository.protection.ProtectNotification(eventID, "destination", normalizedDestination)
		if err != nil {
			return err
		}
		secretCiphertext, secretVersion, err := repository.protection.ProtectNotification(eventID, "secret", code)
		if err != nil {
			return err
		}
		if keyVersion != secretVersion {
			return ErrVerificationCipherCorrupt
		}
		challenge := verificationChallengeRecord{
			ChallengeID: challengeID, BusinessEventID: strings.TrimSpace(request.BusinessEventID), BindingHash: bindingHash,
			Purpose: string(request.Purpose), UserID: strings.TrimSpace(request.UserID), TenantID: strings.TrimSpace(request.TenantID),
			FlowID: strings.TrimSpace(request.FlowID), Channel: string(request.Channel), DestinationHash: destinationHash,
			MaskedDestination: masked, CodeHash: repository.protection.HashCode(challengeID, code),
			MaxAttempts: uint32(policy.MaxVerificationTries), ExpiresAt: now.Add(policy.CodeTTL), CreatedAt: now,
		}
		result := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&challenge)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			existing, found, err := findChallengeByBusinessEvent(ctx, tx, request.BusinessEventID, true)
			if err != nil {
				return err
			}
			if !found || !constantVerificationEqual(existing.BindingHash, bindingHash) {
				return domain.ErrVerificationConflict
			}
			receipt, err = challengeReceipt(ctx, tx, existing)
			return err
		}
		outbox := securityNotificationOutboxRecord{
			EventID: eventID, BusinessEventID: strings.TrimSpace(request.BusinessEventID), ChallengeID: challengeID,
			Kind: string(domain.SecurityNotificationVerificationCode), Purpose: string(request.Purpose),
			UserID: strings.TrimSpace(request.UserID), TenantID: strings.TrimSpace(request.TenantID), FlowID: strings.TrimSpace(request.FlowID),
			Channel: string(request.Channel), DestinationHash: destinationHash, MaskedDestination: masked,
			DestinationCiphertext: destinationCiphertext, SecretCiphertext: secretCiphertext, KeyVersion: keyVersion,
			State: domain.NotificationStatePending, ExpiresAt: challenge.ExpiresAt, CreatedAt: now, UpdatedAt: now,
		}
		outbox.BindingHash = repository.notificationBinding(outbox, code)
		if err := tx.WithContext(ctx).Create(&outbox).Error; err != nil {
			return err
		}
		receipt = domain.VerificationChallengeReceipt{
			ChallengeID: challengeID, NotificationEventID: eventID, MaskedDestination: masked,
			ExpiresAt: challenge.ExpiresAt, DeliveryState: domain.NotificationStatePending,
		}
		return nil
	})
	return receipt, err
}

func (repository *VerificationRepository) VerifyChallenge(ctx context.Context, request domain.VerifyChallengeRequest, policy domain.VerificationPolicy) (domain.OneTimeAuthorization, error) {
	if repository == nil || repository.database == nil || repository.protection == nil || strings.TrimSpace(request.Code) == "" {
		return domain.OneTimeAuthorization{}, domain.ErrVerificationInvalid
	}
	if err := policy.Validate(); err != nil {
		return domain.OneTimeAuthorization{}, err
	}
	destinationHash, _, err := repository.protection.DestinationHash(request.Channel, request.Destination)
	if err != nil {
		return domain.OneTimeAuthorization{}, err
	}
	expectedBinding := repository.protection.BindingHash(request.Purpose, request.UserID, request.TenantID, request.FlowID, request.Channel, destinationHash)
	var authorization domain.OneTimeAuthorization
	var outcome error
	err = repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		var challenge verificationChallengeRecord
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("challenge_id = ?", strings.TrimSpace(request.ChallengeID)).First(&challenge).Error; err != nil {
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
		if !constantVerificationEqual(challenge.CodeHash, repository.protection.HashCode(challenge.ChallengeID, request.Code)) {
			challenge.Attempts++
			if err := tx.WithContext(ctx).Model(&verificationChallengeRecord{}).Where("challenge_id = ?", challenge.ChallengeID).Update("attempts", challenge.Attempts).Error; err != nil {
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
		if err := tx.WithContext(ctx).Model(&verificationChallengeRecord{}).
			Where("challenge_id = ? AND consumed_at IS NULL", challenge.ChallengeID).
			Updates(map[string]any{"consumed_at": consumedAt}).Error; err != nil {
			return err
		}
		record := oneTimeAuthorizationRecord{
			AuthorizationHash: repository.protection.HashAuthorization(rawAuthorization), ChallengeID: challenge.ChallengeID,
			BindingHash: challenge.BindingHash, Purpose: challenge.Purpose, UserID: challenge.UserID, TenantID: challenge.TenantID,
			FlowID: challenge.FlowID, Channel: challenge.Channel, DestinationHash: challenge.DestinationHash,
			ExpiresAt: now.Add(policy.AuthorizationTTL), CreatedAt: now,
		}
		if err := tx.WithContext(ctx).Create(&record).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Model(&securityNotificationOutboxRecord{}).
			Where("challenge_id = ? AND state IN ?", challenge.ChallengeID, []string{domain.NotificationStatePending, domain.NotificationStateFailed}).
			Updates(map[string]any{
				"state": domain.NotificationStateCancelled, "destination_ciphertext": "", "secret_ciphertext": "",
				"failure_code": "CHALLENGE_CONSUMED", "updated_at": now,
			}).Error; err != nil {
			return err
		}
		authorization = domain.OneTimeAuthorization{Code: rawAuthorization, ExpiresAt: record.ExpiresAt}
		return nil
	})
	if err != nil {
		return domain.OneTimeAuthorization{}, err
	}
	if outcome != nil {
		return domain.OneTimeAuthorization{}, outcome
	}
	return authorization, nil
}

func (repository *VerificationRepository) ConsumeOneTimeAuthorization(ctx context.Context, request domain.ConsumeAuthorizationRequest) (domain.AuthorizationConsumptionReceipt, error) {
	if repository == nil || repository.database == nil || repository.protection == nil || strings.TrimSpace(request.Code) == "" {
		return domain.AuthorizationConsumptionReceipt{}, domain.ErrVerificationInvalid
	}
	destinationHash, _, err := repository.protection.DestinationHash(request.Channel, request.Destination)
	if err != nil {
		return domain.AuthorizationConsumptionReceipt{}, err
	}
	expectedBinding := repository.protection.BindingHash(request.Purpose, request.UserID, request.TenantID, request.FlowID, request.Channel, destinationHash)
	hash := repository.protection.HashAuthorization(request.Code)
	var receipt domain.AuthorizationConsumptionReceipt
	var outcome error
	err = repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		var authorization oneTimeAuthorizationRecord
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("authorization_hash = ?", hash).First(&authorization).Error; err != nil {
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
		consumedAt := now
		result := tx.WithContext(ctx).Model(&oneTimeAuthorizationRecord{}).
			Where("authorization_hash = ? AND consumed_at IS NULL", hash).Update("consumed_at", consumedAt)
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

func (repository *VerificationRepository) EnqueueSecurityNotification(ctx context.Context, request domain.SecurityNotificationRequest) (domain.NotificationDeliveryReceipt, error) {
	if repository == nil || repository.database == nil || repository.protection == nil {
		return domain.NotificationDeliveryReceipt{}, domain.ErrNotificationUnavailable
	}
	if strings.TrimSpace(request.BusinessEventID) == "" || !request.Kind.Valid() || !request.Purpose.Valid() || !request.Channel.Valid() || strings.TrimSpace(request.UserID) == "" || strings.TrimSpace(request.Destination) == "" || request.ExpiresAt.IsZero() {
		return domain.NotificationDeliveryReceipt{}, domain.ErrVerificationInvalid
	}
	if notificationKindRequiresSecret(request.Kind) && strings.TrimSpace(request.Secret) == "" {
		return domain.NotificationDeliveryReceipt{}, domain.ErrVerificationInvalid
	}
	destinationHash, normalizedDestination, err := repository.protection.DestinationHash(request.Channel, request.Destination)
	if err != nil {
		return domain.NotificationDeliveryReceipt{}, err
	}
	eventID := stableSecurityEventID(request.BusinessEventID)
	destinationCiphertext, keyVersion, err := repository.protection.ProtectNotification(eventID, "destination", normalizedDestination)
	if err != nil {
		return domain.NotificationDeliveryReceipt{}, err
	}
	secretCiphertext := ""
	if request.Secret != "" {
		secretCiphertext, _, err = repository.protection.ProtectNotification(eventID, "secret", request.Secret)
		if err != nil {
			return domain.NotificationDeliveryReceipt{}, err
		}
	}
	masked := repository.protection.MaskDestination(request.Channel, normalizedDestination)
	var receipt domain.NotificationDeliveryReceipt
	err = repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		if !request.ExpiresAt.After(now) {
			return domain.ErrVerificationExpired
		}
		var existing securityNotificationOutboxRecord
		err = tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("business_event_id = ?", strings.TrimSpace(request.BusinessEventID)).First(&existing).Error
		if err == nil {
			expectedBinding := repository.notificationBinding(existing, request.Secret)
			if !constantVerificationEqual(existing.BindingHash, expectedBinding) {
				return domain.ErrVerificationConflict
			}
			receipt = notificationReceipt(existing)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		record := securityNotificationOutboxRecord{
			EventID: eventID, BusinessEventID: strings.TrimSpace(request.BusinessEventID),
			Kind: string(request.Kind), Purpose: string(request.Purpose), UserID: strings.TrimSpace(request.UserID), TenantID: strings.TrimSpace(request.TenantID),
			FlowID: strings.TrimSpace(request.FlowID), Channel: string(request.Channel), DestinationHash: destinationHash, MaskedDestination: masked,
			DestinationCiphertext: destinationCiphertext, SecretCiphertext: secretCiphertext, KeyVersion: keyVersion,
			State: domain.NotificationStatePending, ExpiresAt: request.ExpiresAt.UTC(), CreatedAt: now, UpdatedAt: now,
		}
		record.BindingHash = repository.notificationBinding(record, request.Secret)
		result := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&record)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			if err := tx.WithContext(ctx).Where("business_event_id = ?", record.BusinessEventID).First(&existing).Error; err != nil {
				return err
			}
			if !constantVerificationEqual(existing.BindingHash, bindingHash) {
				return domain.ErrVerificationConflict
			}
			receipt = notificationReceipt(existing)
			return nil
		}
		receipt = notificationReceipt(record)
		return nil
	})
	return receipt, err
}

func (repository *VerificationRepository) ClaimSecurityNotification(ctx context.Context, eventID string) (domain.SecurityNotificationClaim, domain.NotificationDeliveryReceipt, error) {
	if repository == nil || repository.database == nil || repository.protection == nil || strings.TrimSpace(eventID) == "" {
		return domain.SecurityNotificationClaim{}, domain.NotificationDeliveryReceipt{}, domain.ErrNotificationUnavailable
	}
	var claim domain.SecurityNotificationClaim
	var receipt domain.NotificationDeliveryReceipt
	var outcome error
	err := repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		var record securityNotificationOutboxRecord
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", strings.TrimSpace(eventID)).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				outcome = domain.ErrNotificationUnavailable
				return nil
			}
			return err
		}
		receipt = notificationReceipt(record)
		switch record.State {
		case domain.NotificationStateDelivered:
			return nil
		case domain.NotificationStateSending:
			outcome = domain.ErrNotificationInFlight
			return nil
		case domain.NotificationStateCancelled:
			outcome = domain.ErrNotificationUnavailable
			return nil
		case domain.NotificationStatePending, domain.NotificationStateFailed:
		default:
			outcome = domain.ErrNotificationUnavailable
			return nil
		}
		if !record.ExpiresAt.After(now) {
			record.State = domain.NotificationStateFailed
			record.FailureCode = "NOTIFICATION_EXPIRED"
			record.DestinationCiphertext = ""
			record.SecretCiphertext = ""
			record.UpdatedAt = now
			if err := tx.WithContext(ctx).Model(&securityNotificationOutboxRecord{}).Where("event_id = ?", record.EventID).Updates(map[string]any{
				"state": record.State, "failure_code": record.FailureCode, "destination_ciphertext": "", "secret_ciphertext": "", "updated_at": now,
			}).Error; err != nil {
				return err
			}
			receipt = notificationReceipt(record)
			outcome = domain.ErrNotificationUnavailable
			return nil
		}
		destination, err := repository.protection.DecryptNotification(record.EventID, "destination", record.DestinationCiphertext, record.KeyVersion)
		if err != nil {
			return err
		}
		secret := ""
		if record.SecretCiphertext != "" {
			secret, err = repository.protection.DecryptNotification(record.EventID, "secret", record.SecretCiphertext, record.KeyVersion)
			if err != nil {
				return err
			}
		}
		expectedBinding := repository.notificationBinding(record, secret)
		if !constantVerificationEqual(record.BindingHash, expectedBinding) {
			return ErrVerificationCipherCorrupt
		}
		record.Attempts++
		record.State = domain.NotificationStateSending
		record.UpdatedAt = now
		if err := tx.WithContext(ctx).Model(&securityNotificationOutboxRecord{}).Where("event_id = ?", record.EventID).
			Updates(map[string]any{"state": record.State, "attempts": record.Attempts, "failure_code": "", "updated_at": now}).Error; err != nil {
			return err
		}
		claim = domain.SecurityNotificationClaim{
			EventID: record.EventID, BusinessEventID: record.BusinessEventID, Kind: domain.SecurityNotificationKind(record.Kind),
			Purpose: domain.VerificationPurpose(record.Purpose), UserID: record.UserID, TenantID: record.TenantID, FlowID: record.FlowID,
			Channel: domain.SecurityNotificationChannel(record.Channel), Destination: destination, Secret: secret,
			ExpiresAt: record.ExpiresAt, Attempt: record.Attempts,
		}
		receipt = notificationReceipt(record)
		return nil
	})
	if err != nil {
		return domain.SecurityNotificationClaim{}, domain.NotificationDeliveryReceipt{}, err
	}
	if outcome != nil {
		return domain.SecurityNotificationClaim{}, receipt, outcome
	}
	return claim, receipt, nil
}

func (repository *VerificationRepository) CompleteSecurityNotification(ctx context.Context, claim domain.SecurityNotificationClaim, providerReceipt, failureCode string, delivered bool) (domain.NotificationDeliveryReceipt, error) {
	if repository == nil || repository.database == nil || strings.TrimSpace(claim.EventID) == "" || claim.Attempt == 0 {
		return domain.NotificationDeliveryReceipt{}, domain.ErrNotificationUnavailable
	}
	var receipt domain.NotificationDeliveryReceipt
	var outcome error
	err := repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		var record securityNotificationOutboxRecord
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", claim.EventID).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				outcome = domain.ErrNotificationUnavailable
				return nil
			}
			return err
		}
		if record.State == domain.NotificationStateDelivered {
			receipt = notificationReceipt(record)
			return nil
		}
		if record.State != domain.NotificationStateSending || record.Attempts != claim.Attempt {
			outcome = domain.ErrNotificationInFlight
			return nil
		}
		values := map[string]any{"updated_at": now}
		if delivered {
			record.State = domain.NotificationStateDelivered
			record.ProviderReceipt = strings.TrimSpace(providerReceipt)
			record.FailureCode = ""
			record.DeliveredAt = &now
			record.DestinationCiphertext = ""
			record.SecretCiphertext = ""
			values["state"] = record.State
			values["provider_receipt"] = record.ProviderReceipt
			values["failure_code"] = ""
			values["delivered_at"] = now
			values["destination_ciphertext"] = ""
			values["secret_ciphertext"] = ""
		} else {
			record.State = domain.NotificationStateFailed
			record.FailureCode = sanitizeFailureCode(failureCode)
			values["state"] = record.State
			values["failure_code"] = record.FailureCode
		}
		if err := tx.WithContext(ctx).Model(&securityNotificationOutboxRecord{}).Where("event_id = ?", record.EventID).Updates(values).Error; err != nil {
			return err
		}
		receipt = notificationReceipt(record)
		return nil
	})
	if err != nil {
		return domain.NotificationDeliveryReceipt{}, err
	}
	if outcome != nil {
		return receipt, outcome
	}
	return receipt, nil
}

func challengeReceipt(ctx context.Context, tx *gorm.DB, challenge verificationChallengeRecord) (domain.VerificationChallengeReceipt, error) {
	var outbox securityNotificationOutboxRecord
	if err := tx.WithContext(ctx).Where("challenge_id = ?", challenge.ChallengeID).First(&outbox).Error; err != nil {
		return domain.VerificationChallengeReceipt{}, err
	}
	return domain.VerificationChallengeReceipt{
		ChallengeID: challenge.ChallengeID, NotificationEventID: outbox.EventID,
		MaskedDestination: challenge.MaskedDestination, ExpiresAt: challenge.ExpiresAt, DeliveryState: outbox.State,
	}, nil
}

func findChallengeByBusinessEvent(ctx context.Context, tx *gorm.DB, businessEventID string, lock bool) (verificationChallengeRecord, bool, error) {
	var challenge verificationChallengeRecord
	query := tx.WithContext(ctx)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.Where("business_event_id = ?", strings.TrimSpace(businessEventID)).First(&challenge).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return verificationChallengeRecord{}, false, nil
	}
	if err != nil {
		return verificationChallengeRecord{}, false, err
	}
	return challenge, true, nil
}

func (repository *VerificationRepository) notificationBinding(record securityNotificationOutboxRecord, secret string) string {
	return repository.protection.NotificationBindingHash(
		domain.SecurityNotificationKind(record.Kind), domain.VerificationPurpose(record.Purpose),
		record.UserID, record.TenantID, record.FlowID, domain.SecurityNotificationChannel(record.Channel),
		record.DestinationHash, secret, record.ExpiresAt,
	)
}

func notificationReceipt(record securityNotificationOutboxRecord) domain.NotificationDeliveryReceipt {
	return domain.NotificationDeliveryReceipt{
		EventID: record.EventID, State: record.State, ProviderReceipt: record.ProviderReceipt,
		FailureCode: record.FailureCode, Attempt: record.Attempts, DeliveredAt: record.DeliveredAt,
	}
}

func verificationDatabaseNow(ctx context.Context, database *gorm.DB) (time.Time, error) {
	var row struct {
		Now time.Time `gorm:"column:now"`
	}
	if err := database.WithContext(ctx).Raw("SELECT UTC_TIMESTAMP(6) AS now").Scan(&row).Error; err != nil {
		return time.Time{}, err
	}
	if row.Now.IsZero() {
		return time.Time{}, errors.New("access persistence: database time unavailable")
	}
	return row.Now.UTC(), nil
}

func constantVerificationEqual(left, right string) bool {
	if len(left) == 0 || len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func notificationKindRequiresSecret(kind domain.SecurityNotificationKind) bool {
	switch kind {
	case domain.SecurityNotificationVerificationCode, domain.SecurityNotificationInitialCredential, domain.SecurityNotificationPasswordReset:
		return true
	default:
		return false
	}
}

func sanitizeFailureCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "DELIVERY_FAILED"
	}
	if len(value) > 64 {
		value = value[:64]
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '_':
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "DELIVERY_FAILED"
	}
	return builder.String()
}
