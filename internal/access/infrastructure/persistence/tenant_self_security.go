package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTenantSelfSecurityUnavailable = errors.New("access: tenant self security unavailable")
	ErrTenantSelfContactUnavailable  = errors.New("access: tenant self contact unavailable")
	ErrTenantSelfContactUnchanged    = errors.New("access: tenant self contact unchanged")
	ErrTenantSelfDeletionIrreversible = errors.New("access: self-deleted membership cannot be restored")
)

type TenantSelfSecurityService struct {
	database               *gorm.DB
	contactProtection      *ContactProtection
	verificationProtection *VerificationProtection
	policy                 domain.VerificationPolicy
}

type TenantSelfChallengeReceipt struct {
	ChallengeID       string
	FlowID            string
	MaskedDestination string
	ExpiresAt         time.Time
	DeliveryState     string
	MemberVersion     uint64
	ResendAfter       time.Duration
}

type TenantSelfContactChangeReceipt struct {
	TenantID            string
	UserID              string
	MaskedEmail         string
	MaskedPhone         string
	Version             uint64
	NotificationEventID string
	NotificationState   string
}

type TenantSelfDeletionReceipt struct {
	TenantID            string
	UserID              string
	Version             uint64
	DeletedAt           time.Time
	NotificationEventID string
	NotificationState   string
}

func NewTenantSelfSecurityService(
	database *gorm.DB,
	contactProtection *ContactProtection,
	verificationProtection *VerificationProtection,
	policy domain.VerificationPolicy,
) (*TenantSelfSecurityService, error) {
	if database == nil || contactProtection == nil || verificationProtection == nil {
		return nil, ErrTenantSelfSecurityUnavailable
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &TenantSelfSecurityService{
		database: database, contactProtection: contactProtection,
		verificationProtection: verificationProtection, policy: policy,
	}, nil
}

func (service *TenantSelfSecurityService) RequestContactChange(
	ctx context.Context,
	userID, tenantID, currentPassword string,
	channel domain.SecurityNotificationChannel,
	destination, flowID, businessEventID string,
) (TenantSelfChallengeReceipt, error) {
	if service == nil || service.database == nil || service.contactProtection == nil || service.verificationProtection == nil {
		return TenantSelfChallengeReceipt{}, ErrTenantSelfSecurityUnavailable
	}
	userID, tenantID = strings.TrimSpace(userID), strings.TrimSpace(tenantID)
	flowID, businessEventID = strings.TrimSpace(flowID), strings.TrimSpace(businessEventID)
	if userID == "" || tenantID == "" || currentPassword == "" || flowID == "" || businessEventID == "" || !channel.Valid() {
		return TenantSelfChallengeReceipt{}, domain.ErrVerificationInvalid
	}
	if err := verifyUserPasswordByID(ctx, service.database, userID, currentPassword); err != nil {
		if errors.Is(err, ErrInvalidUserCredentials) {
			return TenantSelfChallengeReceipt{}, ErrCurrentPasswordInvalid
		}
		return TenantSelfChallengeReceipt{}, err
	}
	_, normalized, err := service.verificationProtection.DestinationHash(channel, destination)
	if err != nil {
		return TenantSelfChallengeReceipt{}, err
	}
	var member membershipRecord
	if err := service.database.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.TenantMemberStatusActive).
		First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return TenantSelfChallengeReceipt{}, ports.ErrTenantMemberNotFound
		}
		return TenantSelfChallengeReceipt{}, err
	}
	current, err := service.memberContact(member, channel)
	if err != nil {
		return TenantSelfChallengeReceipt{}, err
	}
	if current == normalized {
		return TenantSelfChallengeReceipt{}, ErrTenantSelfContactUnchanged
	}
	memberRepository := &TenantMemberRepository{database: service.database, contactProtection: service.contactProtection}
	switch channel {
	case domain.SecurityNotificationEmail:
		err = memberRepository.assertMemberContactsAvailable(ctx, tenantID, userID, normalized, "")
	case domain.SecurityNotificationSMS:
		err = memberRepository.assertMemberContactsAvailable(ctx, tenantID, userID, "", normalized)
	default:
		err = domain.ErrVerificationInvalid
	}
	if err != nil {
		return TenantSelfChallengeReceipt{}, err
	}
	verification, err := NewVerificationRepository(service.database, service.verificationProtection)
	if err != nil {
		return TenantSelfChallengeReceipt{}, err
	}
	challenge, err := verification.CreateVerificationChallenge(ctx, domain.VerificationChallengeRequest{
		BusinessEventID: businessEventID,
		FlowID:          flowID,
		Purpose:         domain.VerificationPurposeContactChange,
		UserID:          userID,
		TenantID:        tenantID,
		Channel:         channel,
		Destination:     normalized,
	}, service.policy)
	if err != nil {
		return TenantSelfChallengeReceipt{}, err
	}
	return TenantSelfChallengeReceipt{
		ChallengeID: challenge.ChallengeID, FlowID: flowID,
		MaskedDestination: challenge.MaskedDestination, ExpiresAt: challenge.ExpiresAt,
		DeliveryState: challenge.DeliveryState, MemberVersion: member.Version, ResendAfter: service.policy.ResendInterval,
	}, nil
}

func (service *TenantSelfSecurityService) RequestTenantDeletion(
	ctx context.Context,
	userID, tenantID string,
	channel domain.SecurityNotificationChannel,
	flowID, businessEventID string,
) (TenantSelfChallengeReceipt, error) {
	if service == nil || service.database == nil || service.verificationProtection == nil {
		return TenantSelfChallengeReceipt{}, ErrTenantSelfSecurityUnavailable
	}
	userID, tenantID = strings.TrimSpace(userID), strings.TrimSpace(tenantID)
	flowID, businessEventID = strings.TrimSpace(flowID), strings.TrimSpace(businessEventID)
	if userID == "" || tenantID == "" || flowID == "" || businessEventID == "" || !channel.Valid() {
		return TenantSelfChallengeReceipt{}, domain.ErrVerificationInvalid
	}
	var member membershipRecord
	var destination string
	if err := service.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.TenantMemberStatusActive).
			First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrTenantMemberNotFound
			}
			return err
		}
		roleRepository, err := NewTenantRoleRepository(tx)
		if err != nil {
			return err
		}
		if err := roleRepository.AssertMemberCanDeactivate(ctx, tenantID, userID); err != nil {
			return err
		}
		destination, err = service.memberContact(member, channel)
		if err != nil {
			return err
		}
		if strings.TrimSpace(destination) == "" {
			return ErrTenantSelfContactUnavailable
		}
		return nil
	}); err != nil {
		return TenantSelfChallengeReceipt{}, err
	}
	verification, err := NewVerificationRepository(service.database, service.verificationProtection)
	if err != nil {
		return TenantSelfChallengeReceipt{}, err
	}
	challenge, err := verification.CreateVerificationChallenge(ctx, domain.VerificationChallengeRequest{
		BusinessEventID: businessEventID,
		FlowID:          flowID,
		Purpose:         domain.VerificationPurposeAccountDeletion,
		UserID:          userID,
		TenantID:        tenantID,
		Channel:         channel,
		Destination:     destination,
	}, service.policy)
	if err != nil {
		return TenantSelfChallengeReceipt{}, err
	}
	return TenantSelfChallengeReceipt{
		ChallengeID: challenge.ChallengeID, FlowID: flowID,
		MaskedDestination: challenge.MaskedDestination, ExpiresAt: challenge.ExpiresAt,
		DeliveryState: challenge.DeliveryState, MemberVersion: member.Version, ResendAfter: service.policy.ResendInterval,
	}, nil
}

func (service *TenantSelfSecurityService) CompleteContactChange(
	ctx context.Context,
	userID, tenantID string,
	channel domain.SecurityNotificationChannel,
	challengeID, flowID, code string,
	expectedVersion uint64,
	requestRef string,
) (TenantSelfContactChangeReceipt, error) {
	if service == nil || service.database == nil || expectedVersion == 0 {
		return TenantSelfContactChangeReceipt{}, domain.ErrVerificationInvalid
	}
	userID, tenantID = strings.TrimSpace(userID), strings.TrimSpace(tenantID)
	if userID == "" || tenantID == "" || !channel.Valid() {
		return TenantSelfContactChangeReceipt{}, domain.ErrVerificationInvalid
	}
	var receipt TenantSelfContactChangeReceipt
	var outcome error
	err := service.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		verified, challengeOutcome, err := service.verifyChallengeTx(
			ctx, tx, domain.VerificationPurposeContactChange, userID, tenantID,
			channel, challengeID, flowID, code,
		)
		if err != nil {
			return err
		}
		if challengeOutcome != nil {
			outcome = challengeOutcome
			return nil
		}
		var member membershipRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				outcome = ports.ErrTenantMemberNotFound
				return nil
			}
			return err
		}
		if member.Status != domain.TenantMemberStatusActive || member.Version != expectedVersion || member.SelfDeletedAt != nil {
			outcome = ports.ErrTenantMemberConflict
			return nil
		}
		memberRepository := &TenantMemberRepository{database: tx, contactProtection: service.contactProtection}
		switch channel {
		case domain.SecurityNotificationEmail:
			if err := memberRepository.assertMemberContactsAvailable(ctx, tenantID, userID, verified.Destination, ""); err != nil {
				outcome = err
				return nil
			}
		case domain.SecurityNotificationSMS:
			if err := memberRepository.assertMemberContactsAvailable(ctx, tenantID, userID, "", verified.Destination); err != nil {
				outcome = err
				return nil
			}
		}
		updates, err := service.protectedContactUpdates(channel, verified.Destination)
		if err != nil {
			return err
		}
		updates["version"] = gorm.Expr("version + 1")
		updates["updated_at"] = verified.Now
		result := tx.Model(&membershipRecord{}).
			Where("tenant_id = ? AND user_id = ? AND version = ? AND status = ?", tenantID, userID, expectedVersion, domain.TenantMemberStatusActive).
			Updates(updates)
		if result.Error != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(result.Error, &mysqlErr) && mysqlErr.Number == 1062 {
				outcome = ports.ErrTenantMemberContactConflict
				return nil
			}
			return result.Error
		}
		if result.RowsAffected != 1 {
			outcome = ports.ErrTenantMemberConflict
			return nil
		}
		if err := service.consumeVerifiedChallengeTx(ctx, tx, verified); err != nil {
			return err
		}
		notification, err := service.stageSecurityNotificationTx(ctx, tx, domain.SecurityNotificationRequest{
			BusinessEventID: "tenant-self/contact-change-complete/" + verified.Challenge.ChallengeID,
			Kind:            domain.SecurityNotificationContactChanged,
			Purpose:         domain.VerificationPurposeContactChange,
			UserID:          userID,
			TenantID:        tenantID,
			FlowID:          flowID,
			Channel:         channel,
			Destination:     verified.Destination,
			ExpiresAt:       verified.Now.Add(service.policy.AuthorizationTTL),
		}, verified.Now)
		if err != nil {
			return err
		}
		if err := service.stageSelfSecurityAuditTx(ctx, tx, "tenant.personal.contact_change", tenantID, userID, verified.Challenge.ChallengeID, requestRef, notification.EventID, verified.Now); err != nil {
			return err
		}
		maskedEmail, maskedPhone, err := service.maskedMemberContactsAfterUpdate(member, channel, verified.Destination)
		if err != nil {
			return err
		}
		receipt = TenantSelfContactChangeReceipt{
			TenantID: tenantID, UserID: userID, MaskedEmail: maskedEmail, MaskedPhone: maskedPhone,
			Version: expectedVersion + 1, NotificationEventID: notification.EventID, NotificationState: notification.State,
		}
		return nil
	})
	if err != nil {
		return TenantSelfContactChangeReceipt{}, err
	}
	if outcome != nil {
		return TenantSelfContactChangeReceipt{}, outcome
	}
	return receipt, nil
}

func (service *TenantSelfSecurityService) CompleteTenantDeletion(
	ctx context.Context,
	userID, tenantID string,
	channel domain.SecurityNotificationChannel,
	challengeID, flowID, code string,
	expectedVersion uint64,
	requestRef string,
) (TenantSelfDeletionReceipt, error) {
	if service == nil || service.database == nil || expectedVersion == 0 {
		return TenantSelfDeletionReceipt{}, domain.ErrVerificationInvalid
	}
	userID, tenantID = strings.TrimSpace(userID), strings.TrimSpace(tenantID)
	if userID == "" || tenantID == "" || !channel.Valid() {
		return TenantSelfDeletionReceipt{}, domain.ErrVerificationInvalid
	}
	var receipt TenantSelfDeletionReceipt
	var outcome error
	err := service.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		verified, challengeOutcome, err := service.verifyChallengeTx(
			ctx, tx, domain.VerificationPurposeAccountDeletion, userID, tenantID,
			channel, challengeID, flowID, code,
		)
		if err != nil {
			return err
		}
		if challengeOutcome != nil {
			outcome = challengeOutcome
			return nil
		}
		var member membershipRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				outcome = ports.ErrTenantMemberNotFound
				return nil
			}
			return err
		}
		if member.Status != domain.TenantMemberStatusActive || member.Version != expectedVersion || member.SelfDeletedAt != nil {
			outcome = ports.ErrTenantMemberConflict
			return nil
		}
		currentDestination, err := service.memberContact(member, channel)
		if err != nil {
			return err
		}
		if currentDestination == "" {
			outcome = ErrTenantSelfContactUnavailable
			return nil
		}
		currentHash, _, err := service.verificationProtection.DestinationHash(channel, currentDestination)
		if err != nil {
			return err
		}
		if !constantVerificationEqual(currentHash, verified.Challenge.DestinationHash) {
			outcome = domain.ErrVerificationInvalid
			return nil
		}
		roleRepository, err := NewTenantRoleRepository(tx)
		if err != nil {
			return err
		}
		if err := roleRepository.AssertMemberCanDeactivate(ctx, tenantID, userID); err != nil {
			outcome = err
			return nil
		}
		notification, err := service.stageSecurityNotificationTx(ctx, tx, domain.SecurityNotificationRequest{
			BusinessEventID: "tenant-self/deletion-complete/" + verified.Challenge.ChallengeID,
			Kind:            domain.SecurityNotificationTenantDeletion,
			Purpose:         domain.VerificationPurposeAccountDeletion,
			UserID:          userID,
			TenantID:        tenantID,
			FlowID:          flowID,
			Channel:         channel,
			Destination:     currentDestination,
			ExpiresAt:       verified.Now.Add(service.policy.AuthorizationTTL),
		}, verified.Now)
		if err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&memberRoleRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&memberSiteRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&memberRemovedRoleSnapshotRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&memberRemovedSiteSnapshotRecord{}).Error; err != nil {
			return err
		}
		if tx.Migrator().HasTable(&memberStatusAppealRecord{}) {
			if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&memberStatusAppealRecord{}).Error; err != nil {
				return err
			}
		}
		result := tx.Model(&membershipRecord{}).
			Where("tenant_id = ? AND user_id = ? AND version = ? AND status = ?", tenantID, userID, expectedVersion, domain.TenantMemberStatusActive).
			Updates(map[string]any{
				"status":            domain.TenantMemberStatusRemoved,
				"name":              "",
				"email":             "",
				"email_ciphertext":  "",
				"email_lookup_hash": nil,
				"email_key_version": "",
				"phone":             "",
				"phone_ciphertext":  "",
				"phone_lookup_hash": nil,
				"phone_key_version": "",
				"employee_id":       "",
				"position":          "",
				"department_id":     "",
				"avatar_asset_ref":  domain.DefaultPersonalAvatarAssetRef,
				"self_deleted_at":   verified.Now,
				"version":           gorm.Expr("version + 1"),
				"updated_at":        verified.Now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			outcome = ports.ErrTenantMemberConflict
			return nil
		}
		if err := tx.Model(&apiTokenRecord{}).
			Where("tenant_id = ? AND user_id = ? AND disabled = ?", tenantID, userID, false).
			Update("disabled", true).Error; err != nil {
			return err
		}
		if err := revokeWebSessionsForTenantMember(ctx, tx, userID, tenantID, "membership_self_deleted"); err != nil {
			return err
		}
		if err := service.consumeVerifiedChallengeTx(ctx, tx, verified); err != nil {
			return err
		}
		if err := service.stageSelfSecurityAuditTx(ctx, tx, "tenant.membership.self_delete", tenantID, userID, verified.Challenge.ChallengeID, requestRef, notification.EventID, verified.Now); err != nil {
			return err
		}
		receipt = TenantSelfDeletionReceipt{
			TenantID: tenantID, UserID: userID, Version: expectedVersion + 1, DeletedAt: verified.Now,
			NotificationEventID: notification.EventID, NotificationState: notification.State,
		}
		return nil
	})
	if err != nil {
		return TenantSelfDeletionReceipt{}, err
	}
	if outcome != nil {
		return TenantSelfDeletionReceipt{}, outcome
	}
	return receipt, nil
}

type tenantVerifiedChallenge struct {
	Challenge   verificationChallengeRecord
	Outbox      securityNotificationOutboxRecord
	Destination string
	Now         time.Time
}

func (service *TenantSelfSecurityService) verifyChallengeTx(
	ctx context.Context,
	tx *gorm.DB,
	purpose domain.VerificationPurpose,
	userID, tenantID string,
	channel domain.SecurityNotificationChannel,
	challengeID, flowID, code string,
) (tenantVerifiedChallenge, error, error) {
	challengeID, flowID, code = strings.TrimSpace(challengeID), strings.TrimSpace(flowID), strings.TrimSpace(code)
	if challengeID == "" || flowID == "" || code == "" {
		return tenantVerifiedChallenge{}, domain.ErrVerificationInvalid, nil
	}
	now, err := verificationDatabaseNow(ctx, tx)
	if err != nil {
		return tenantVerifiedChallenge{}, nil, err
	}
	var challenge verificationChallengeRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("challenge_id = ?", challengeID).First(&challenge).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tenantVerifiedChallenge{}, domain.ErrVerificationInvalid, nil
		}
		return tenantVerifiedChallenge{}, nil, err
	}
	if challenge.Purpose != string(purpose) || challenge.UserID != userID || challenge.TenantID != tenantID ||
		challenge.FlowID != flowID || challenge.Channel != string(channel) {
		return tenantVerifiedChallenge{}, domain.ErrVerificationInvalid, nil
	}
	if challenge.ConsumedAt != nil {
		return tenantVerifiedChallenge{}, domain.ErrVerificationConsumed, nil
	}
	if !challenge.ExpiresAt.After(now) {
		return tenantVerifiedChallenge{}, domain.ErrVerificationExpired, nil
	}
	if challenge.Attempts >= challenge.MaxAttempts {
		return tenantVerifiedChallenge{}, domain.RateLimitError{Reason: "attempt_limit"}, nil
	}
	var outbox securityNotificationOutboxRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("challenge_id = ?", challenge.ChallengeID).First(&outbox).Error; err != nil {
		return tenantVerifiedChallenge{}, nil, err
	}
	destination, err := service.verificationProtection.DecryptNotification(outbox.EventID, "destination", outbox.DestinationCiphertext, outbox.KeyVersion)
	if err != nil {
		return tenantVerifiedChallenge{}, nil, err
	}
	destinationHash, normalized, err := service.verificationProtection.DestinationHash(channel, destination)
	if err != nil {
		return tenantVerifiedChallenge{}, nil, err
	}
	expectedBinding := service.verificationProtection.BindingHash(purpose, userID, tenantID, flowID, channel, destinationHash)
	if !constantVerificationEqual(challenge.DestinationHash, destinationHash) || !constantVerificationEqual(challenge.BindingHash, expectedBinding) {
		return tenantVerifiedChallenge{}, domain.ErrVerificationInvalid, nil
	}
	if !constantVerificationEqual(challenge.CodeHash, service.verificationProtection.HashCode(challenge.ChallengeID, code)) {
		challenge.Attempts++
		if err := tx.Model(&verificationChallengeRecord{}).
			Where("challenge_id = ?", challenge.ChallengeID).
			Update("attempts", challenge.Attempts).Error; err != nil {
			return tenantVerifiedChallenge{}, nil, err
		}
		if challenge.Attempts >= challenge.MaxAttempts {
			return tenantVerifiedChallenge{}, domain.RateLimitError{Reason: "attempt_limit"}, nil
		}
		return tenantVerifiedChallenge{}, domain.ErrVerificationInvalid, nil
	}
	return tenantVerifiedChallenge{Challenge: challenge, Outbox: outbox, Destination: normalized, Now: now}, nil, nil
}

func (service *TenantSelfSecurityService) consumeVerifiedChallengeTx(ctx context.Context, tx *gorm.DB, verified tenantVerifiedChallenge) error {
	rawAuthorization, err := randomVerificationSecret(32)
	if err != nil {
		return err
	}
	consumedAt := verified.Now
	authorization := oneTimeAuthorizationRecord{
		AuthorizationHash: service.verificationProtection.HashAuthorization(rawAuthorization),
		ChallengeID:       verified.Challenge.ChallengeID,
		BindingHash:       verified.Challenge.BindingHash,
		Purpose:           verified.Challenge.Purpose,
		UserID:            verified.Challenge.UserID,
		TenantID:          verified.Challenge.TenantID,
		FlowID:            verified.Challenge.FlowID,
		Channel:           verified.Challenge.Channel,
		DestinationHash:   verified.Challenge.DestinationHash,
		ExpiresAt:         canonicalVerificationTime(verified.Now.Add(service.policy.AuthorizationTTL)),
		ConsumedAt:        &consumedAt,
		CreatedAt:         canonicalVerificationTime(verified.Now),
	}
	if err := tx.Create(&authorization).Error; err != nil {
		return err
	}
	result := tx.Model(&verificationChallengeRecord{}).
		Where("challenge_id = ? AND consumed_at IS NULL", verified.Challenge.ChallengeID).
		Update("consumed_at", consumedAt)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return domain.ErrVerificationConsumed
	}
	return tx.Model(&securityNotificationOutboxRecord{}).
		Where("challenge_id = ? AND state IN ?", verified.Challenge.ChallengeID, []string{
			domain.NotificationStatePending,
			domain.NotificationStateFailed,
		}).
		Updates(map[string]any{
			"state":                  domain.NotificationStateCancelled,
			"destination_ciphertext": "",
			"secret_ciphertext":      "",
			"failure_code":           "VERIFICATION_CONSUMED",
			"updated_at":             verified.Now,
		}).Error
}

func (service *TenantSelfSecurityService) stageSecurityNotificationTx(
	ctx context.Context,
	tx *gorm.DB,
	request domain.SecurityNotificationRequest,
	now time.Time,
) (domain.NotificationDeliveryReceipt, error) {
	if strings.TrimSpace(request.BusinessEventID) == "" || !request.Kind.Valid() || !request.Purpose.Valid() ||
		!request.Channel.Valid() || strings.TrimSpace(request.UserID) == "" || strings.TrimSpace(request.Destination) == "" ||
		request.ExpiresAt.IsZero() || !request.ExpiresAt.After(now) {
		return domain.NotificationDeliveryReceipt{}, domain.ErrVerificationInvalid
	}
	destinationHash, normalizedDestination, err := service.verificationProtection.DestinationHash(request.Channel, request.Destination)
	if err != nil {
		return domain.NotificationDeliveryReceipt{}, err
	}
	eventID := stableSecurityEventID(request.BusinessEventID)
	destinationCiphertext, keyVersion, err := service.verificationProtection.ProtectNotification(eventID, "destination", normalizedDestination)
	if err != nil {
		return domain.NotificationDeliveryReceipt{}, err
	}
	record := securityNotificationOutboxRecord{
		EventID: eventID, BusinessEventID: strings.TrimSpace(request.BusinessEventID),
		Kind: string(request.Kind), Purpose: string(request.Purpose), UserID: strings.TrimSpace(request.UserID),
		TenantID: strings.TrimSpace(request.TenantID), FlowID: strings.TrimSpace(request.FlowID),
		Channel: string(request.Channel), DestinationHash: destinationHash,
		MaskedDestination: service.verificationProtection.MaskDestination(request.Channel, normalizedDestination),
		DestinationCiphertext: destinationCiphertext, KeyVersion: keyVersion,
		State: domain.NotificationStatePending, ExpiresAt: canonicalVerificationTime(request.ExpiresAt),
		CreatedAt: canonicalVerificationTime(now), UpdatedAt: canonicalVerificationTime(now),
	}
	repository := &VerificationRepository{database: tx, protection: service.verificationProtection}
	record.BindingHash = repository.notificationBinding(record, "")
	var existing securityNotificationOutboxRecord
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("business_event_id = ?", record.BusinessEventID).First(&existing).Error
	if err == nil {
		if !constantVerificationEqual(existing.BindingHash, record.BindingHash) {
			return domain.NotificationDeliveryReceipt{}, domain.ErrVerificationConflict
		}
		return notificationReceipt(existing), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.NotificationDeliveryReceipt{}, err
	}
	if err := tx.Create(&record).Error; err != nil {
		return domain.NotificationDeliveryReceipt{}, err
	}
	return notificationReceipt(record), nil
}

func (service *TenantSelfSecurityService) stageSelfSecurityAuditTx(
	ctx context.Context,
	tx *gorm.DB,
	operationID, tenantID, userID, challengeID, requestRef, receiptRef string,
	now time.Time,
) error {
	seed := operationID + "/" + tenantID + "/" + userID + "/" + challengeID
	auditID := "aud-" + TokenHash(seed)[:40]
	base := auditEventRecord{
		AuditID: auditID, TenantID: tenantID, ActorSubject: "user:" + userID, ActorUserID: userID,
		AuthMethod: AuthMethodWeb, AuthChannel: "web", RequestID: strings.TrimSpace(requestRef),
		IdempotencyRef: TokenHash(strings.TrimSpace(requestRef)), OperationID: operationID,
		Module: "access-management", Target: "user:" + userID, ResourceTenantID: tenantID,
		RequestDigest: TokenHash(seed), ReceiptRef: strings.TrimSpace(receiptRef),
		Risk: domain.AuditRiskHigh, OccurredAt: now,
	}
	attempt := base
	attempt.EventID = "evt-" + TokenHash(auditID+"/attempt")[:40]
	attempt.EventType = domain.AuditEventAttempt
	attempt.Outcome = domain.AuditResultPending
	outcome := base
	outcome.EventID = "evt-" + TokenHash(auditID+"/outcome")[:40]
	outcome.EventType = domain.AuditEventOutcome
	outcome.Outcome = domain.AuditResultSuccess
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&attempt).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&outcome).Error
}

func (service *TenantSelfSecurityService) protectedContactUpdates(
	channel domain.SecurityNotificationChannel,
	destination string,
) (map[string]any, error) {
	switch channel {
	case domain.SecurityNotificationEmail:
		ciphertext, lookup, version, err := service.contactProtection.ProtectEmail(destination)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"email": "", "email_ciphertext": ciphertext, "email_lookup_hash": lookup, "email_key_version": version,
		}, nil
	case domain.SecurityNotificationSMS:
		ciphertext, lookup, version, err := service.contactProtection.ProtectPhone(destination)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"phone": "", "phone_ciphertext": ciphertext, "phone_lookup_hash": lookup, "phone_key_version": version,
		}, nil
	default:
		return nil, domain.ErrVerificationInvalid
	}
}

func (service *TenantSelfSecurityService) memberContact(member membershipRecord, channel domain.SecurityNotificationChannel) (string, error) {
	switch channel {
	case domain.SecurityNotificationEmail:
		if member.EmailCiphertext != "" || member.EmailKeyVersion != "" || member.EmailLookupHash != nil {
			return service.contactProtection.DecryptEmail(member.EmailCiphertext, member.EmailKeyVersion)
		}
		if strings.TrimSpace(member.Email) == "" {
			return "", nil
		}
		return NormalizeEmail(member.Email)
	case domain.SecurityNotificationSMS:
		if member.PhoneCiphertext != "" || member.PhoneKeyVersion != "" || member.PhoneLookupHash != nil {
			return service.contactProtection.DecryptPhone(member.PhoneCiphertext, member.PhoneKeyVersion)
		}
		if strings.TrimSpace(member.Phone) == "" {
			return "", nil
		}
		return NormalizePhone(member.Phone)
	default:
		return "", domain.ErrVerificationInvalid
	}
}

func (service *TenantSelfSecurityService) maskedMemberContactsAfterUpdate(
	member membershipRecord,
	channel domain.SecurityNotificationChannel,
	destination string,
) (string, string, error) {
	email, err := service.memberContact(member, domain.SecurityNotificationEmail)
	if err != nil {
		return "", "", err
	}
	phone, err := service.memberContact(member, domain.SecurityNotificationSMS)
	if err != nil {
		return "", "", err
	}
	if channel == domain.SecurityNotificationEmail {
		email = destination
	} else {
		phone = destination
	}
	return MaskEmail(email), MaskPhone(phone), nil
}
