package application

import (
	"context"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
)

type externalRepoFake struct {
	claim       domain.ExternalTaskClaim
	terminal    string
	failureCode string
	retryable   *bool
	completed   domain.ExternalProviderResult
}

func (r *externalRepoFake) ClaimNextExternalTask(context.Context,string,domain.ExternalDeliveryPolicy)(domain.ExternalTaskClaim,domain.ExternalTaskReceipt,error){return r.claim,domain.ExternalTaskReceipt{TaskID:r.claim.TaskID,State:domain.ExternalTaskStateLeased,Attempt:r.claim.Attempt},nil}
func (r *externalRepoFake) CompleteExternalTask(_ context.Context,_ domain.ExternalTaskClaim,result domain.ExternalProviderResult)(domain.ExternalTaskReceipt,error){r.completed=result;state:=domain.ExternalTaskStateProviderAccepted;if result.Status==domain.ExternalProviderDelivered{state=domain.ExternalTaskStateDelivered};return domain.ExternalTaskReceipt{TaskID:r.claim.TaskID,State:state,Attempt:r.claim.Attempt,ProviderReceipt:result.ReceiptID},nil}
func (r *externalRepoFake) FailExternalTask(_ context.Context,_ domain.ExternalTaskClaim,code string,retry bool,_ domain.ExternalDeliveryPolicy)(domain.ExternalTaskReceipt,error){r.failureCode=code;r.retryable=&retry;state:=domain.ExternalTaskStateManualReview;if retry{state=domain.ExternalTaskStateRetryWait};return domain.ExternalTaskReceipt{TaskID:r.claim.TaskID,State:state,Attempt:r.claim.Attempt,FailureCode:code},nil}
func (r *externalRepoFake) TerminalExternalTask(_ context.Context,_ domain.ExternalTaskClaim,state,code string)(domain.ExternalTaskReceipt,error){r.terminal=state;r.failureCode=code;return domain.ExternalTaskReceipt{TaskID:r.claim.TaskID,State:state,Attempt:r.claim.Attempt,FailureCode:code},nil}

type admissionFake struct{ admission accessdomain.OptionalNotificationDeliveryAdmission; err error; calls int }
func (a *admissionFake) PrepareOptionalNotificationDelivery(context.Context,accessdomain.NotificationPreferenceOwner,accessdomain.NotificationPreferenceChannel)(accessdomain.OptionalNotificationDeliveryAdmission,error){a.calls++;return a.admission,a.err}

type providerFake struct{ result domain.ExternalProviderResult; err error; safe bool; calls int; request domain.ExternalProviderRequest }
func (p *providerFake) SendExternalNotification(_ context.Context,request domain.ExternalProviderRequest)(domain.ExternalProviderResult,error){p.calls++;p.request=request;return p.result,p.err}
func (p *providerFake) ExternalNotificationIdempotent()bool{return p.safe}

type externalFailure struct{ retryable,known bool }
func(externalFailure)Error()string{return "provider failed"}
func(externalFailure)FailureCode()string{return "PROVIDER_TEMPORARY"}
func(e externalFailure)Retryable()bool{return e.retryable}
func(e externalFailure)OutcomeKnown()bool{return e.known}

func externalClaimFixture() domain.ExternalTaskClaim {
	now:=time.Now().UTC()
	return domain.ExternalTaskClaim{TaskID:"task-185",TenantID:"tenant-185",UserID:"user-185",EventID:"event-185",Channel:"email",ConfigurationID:"config-185",ConfigurationVersion:1,GroupID:"site-185",TypeCode:"device.fault",Level:domain.LevelUrgent,TraceID:"trace-185",ReferenceKind:"device",ReferenceID:"machine-185",Attempt:1,WorkerID:"worker-185",LeaseToken:1,LeaseUntil:now.Add(time.Minute)}
}

func externalWorkerFixture(t *testing.T,admission accessdomain.OptionalNotificationDeliveryAdmission,provider *providerFake)(*ExternalDeliveryWorker,*externalRepoFake,*admissionFake){
	t.Helper();repo:=&externalRepoFake{claim:externalClaimFixture()};admitter:=&admissionFake{admission:admission}
	worker,err:=NewExternalDeliveryWorker(ports.ExternalDeliveryDependencies{Tasks:repo,Admission:admitter,Provider:provider},domain.EnterpriseExternalDeliveryPolicy(),"worker-185");if err!=nil{t.Fatal(err)}
	return worker,repo,admitter
}

func TestEnterprise185ExternalWorkerRechecksPreferenceBeforeProvider(t *testing.T){
	provider:=&providerFake{result:domain.ExternalProviderResult{ReceiptID:"provider-185",Status:domain.ExternalProviderAccepted},safe:true}
	worker,repo,admitter:=externalWorkerFixture(t,accessdomain.OptionalNotificationDeliveryAdmission{NotificationPreferenceOwner:accessdomain.NotificationPreferenceOwner{TenantID:"tenant-185",UserID:"user-185"},Channel:accessdomain.NotificationPreferenceEmail,Allowed:false},provider)
	receipt,err:=worker.RunOnce(context.Background())
	if err!=nil||admitter.calls!=1||provider.calls!=0||repo.terminal!=domain.ExternalTaskStateCancelled||repo.failureCode!="PREFERENCE_DENIED"||receipt.State!=domain.ExternalTaskStateCancelled{t.Fatalf("receipt=%+v admission=%d provider=%d terminal=%s code=%s err=%v",receipt,admitter.calls,provider.calls,repo.terminal,repo.failureCode,err)}
}

func TestEnterprise185ExternalWorkerUsesTransientContactAndProviderReceipt(t *testing.T){
	provider:=&providerFake{result:domain.ExternalProviderResult{ReceiptID:"provider-185",Status:domain.ExternalProviderAccepted},safe:true}
	admission:=accessdomain.OptionalNotificationDeliveryAdmission{NotificationPreferenceOwner:accessdomain.NotificationPreferenceOwner{TenantID:"tenant-185",UserID:"user-185"},Channel:accessdomain.NotificationPreferenceEmail,Allowed:true,Destination:"member@example.invalid",PreferenceVersion:2}
	worker,repo,_:=externalWorkerFixture(t,admission,provider)
	receipt,err:=worker.RunOnce(context.Background())
	if err!=nil||provider.calls!=1||provider.request.Destination!="member@example.invalid"||repo.completed.ReceiptID!="provider-185"||receipt.State!=domain.ExternalTaskStateProviderAccepted{t.Fatalf("receipt=%+v request=%+v completed=%+v err=%v",receipt,provider.request,repo.completed,err)}
}

func TestEnterprise185ExternalWorkerMissingContactIsManualReview(t *testing.T){
	provider:=&providerFake{safe:true}
	worker,repo,admitter:=externalWorkerFixture(t,accessdomain.OptionalNotificationDeliveryAdmission{},provider)
	admitter.err=accessdomain.ErrNotificationDeliveryContactUnavailable
	receipt,err:=worker.RunOnce(context.Background())
	if err!=nil||provider.calls!=0||repo.terminal!=domain.ExternalTaskStateManualReview||repo.failureCode!="CONTACT_UNAVAILABLE"||receipt.State!=domain.ExternalTaskStateManualReview{t.Fatalf("receipt=%+v err=%v",receipt,err)}
}

func TestEnterprise185ExternalWorkerUnknownOutcomeRetriesOnlySafeProvider(t *testing.T){
	for _,tc:=range []struct{name string;safe,wantRetry bool}{{"unsafe",false,false},{"safe",true,true}}{
		t.Run(tc.name,func(t *testing.T){
			provider:=&providerFake{err:context.DeadlineExceeded,safe:tc.safe}
			admission:=accessdomain.OptionalNotificationDeliveryAdmission{NotificationPreferenceOwner:accessdomain.NotificationPreferenceOwner{TenantID:"tenant-185",UserID:"user-185"},Channel:accessdomain.NotificationPreferenceEmail,Allowed:true,Destination:"member@example.invalid"}
			worker,repo,_:=externalWorkerFixture(t,admission,provider)
			receipt,err:=worker.RunOnce(context.Background())
			if err!=nil||repo.retryable==nil||*repo.retryable!=tc.wantRetry{t.Fatalf("receipt=%+v retry=%v err=%v",receipt,repo.retryable,err)}
			if tc.wantRetry&&receipt.State!=domain.ExternalTaskStateRetryWait{t.Fatalf("safe provider state=%s",receipt.State)}
			if !tc.wantRetry&&receipt.State!=domain.ExternalTaskStateManualReview{t.Fatalf("unsafe provider state=%s",receipt.State)}
		})
	}
}

func TestEnterprise185ExternalWorkerKnownTransientFailureRetries(t *testing.T){
	provider:=&providerFake{err:externalFailure{retryable:true,known:true},safe:false}
	admission:=accessdomain.OptionalNotificationDeliveryAdmission{NotificationPreferenceOwner:accessdomain.NotificationPreferenceOwner{TenantID:"tenant-185",UserID:"user-185"},Channel:accessdomain.NotificationPreferenceEmail,Allowed:true,Destination:"member@example.invalid"}
	worker,repo,_:=externalWorkerFixture(t,admission,provider)
	receipt,err:=worker.RunOnce(context.Background())
	if err!=nil||repo.retryable==nil||!*repo.retryable||repo.failureCode!="PROVIDER_TEMPORARY"||receipt.State!=domain.ExternalTaskStateRetryWait{t.Fatalf("receipt=%+v retry=%v code=%s err=%v",receipt,repo.retryable,repo.failureCode,err)}
}

var _ ports.ExternalTaskRepository=(*externalRepoFake)(nil)
var _ ports.ExternalNotificationProvider=(*providerFake)(nil)
var _ ports.ExternalNotificationProviderRetrySafety=(*providerFake)(nil)
var _ ports.ExternalNotificationProviderFailure=externalFailure{}
