package application

import (
	"context"
	"errors"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accessports "github.com/hvritual/biz/internal/access/ports"
)

type workerRepo struct {
	claim           accessdomain.ReliableSecurityNotificationClaim
	failedRetryable *bool
	failureCode     string
	completed       bool
}
func (r *workerRepo) EnsureReliableSecurityNotificationSchema(context.Context) error { return nil }
func (r *workerRepo) ClaimNextSecurityNotification(context.Context,string,accessdomain.NotificationRetryPolicy)(accessdomain.ReliableSecurityNotificationClaim,accessdomain.NotificationDeliveryReceipt,error){ return r.claim,accessdomain.NotificationDeliveryReceipt{},nil }
func (r *workerRepo) CompleteReliableSecurityNotification(_ context.Context,_ accessdomain.ReliableSecurityNotificationClaim,_ string)(accessdomain.NotificationDeliveryReceipt,error){ r.completed=true; return accessdomain.NotificationDeliveryReceipt{EventID:r.claim.EventID,State:accessdomain.NotificationStateDelivered,Attempt:r.claim.Attempt},nil }
func (r *workerRepo) FailReliableSecurityNotification(_ context.Context,_ accessdomain.ReliableSecurityNotificationClaim,code string,retry bool,_ accessdomain.NotificationRetryPolicy)(accessdomain.NotificationDeliveryReceipt,error){ r.failedRetryable=&retry;r.failureCode=code;state:=accessdomain.NotificationStateManualReview;if retry{state=accessdomain.NotificationStateRetryWait};return accessdomain.NotificationDeliveryReceipt{EventID:r.claim.EventID,State:state,FailureCode:code,Attempt:r.claim.Attempt},nil }

type workerSender struct { err error; idempotent bool }
func (s workerSender) SendSecurityNotification(context.Context,accessdomain.SecurityNotificationClaim)(string,error){if s.err!=nil{return "",s.err};return "provider-receipt",nil}
func (s workerSender) SecurityNotificationIdempotent() bool { return s.idempotent }

type workerFailure struct{ retryable,known bool }
func (workerFailure) Error() string{return "provider failed"}
func (workerFailure) FailureCode()string{return "PROVIDER_REJECTED"}
func (e workerFailure) Retryable()bool{return e.retryable}
func (e workerFailure) OutcomeKnown()bool{return e.known}

func claimFixture() accessdomain.ReliableSecurityNotificationClaim {
	now:=time.Now().UTC()
	return accessdomain.ReliableSecurityNotificationClaim{SecurityNotificationClaim:accessdomain.SecurityNotificationClaim{EventID:"evt-185",BusinessEventID:"business-185",Kind:accessdomain.SecurityNotificationMemberLifecycle,Purpose:accessdomain.VerificationPurposeMemberLifecycle,UserID:"user-185",TenantID:"tenant-185",Channel:accessdomain.SecurityNotificationEmail,Destination:"masked-test@example.invalid",Attempt:1,ExpiresAt:now.Add(time.Hour)},WorkerID:"worker-185",LeaseToken:1,LeaseUntil:now.Add(time.Minute)}
}

func TestEnterprise185RetryPolicyIsBounded(t *testing.T){
	p:=accessdomain.EnterpriseNotificationRetryPolicy()
	if err:=p.Validate();err!=nil{t.Fatal(err)}
	want:=[]time.Duration{30*time.Second,2*time.Minute,10*time.Minute,30*time.Minute}
	for i,d:=range want{got,ok:=p.NextDelay(uint32(i+1));if !ok||got!=d{t.Fatalf("attempt %d delay=%v ok=%v",i+1,got,ok)}}
	if _,ok:=p.NextDelay(5);ok{t.Fatal("fifth failure retried")}
}

func TestEnterprise185WorkerCompletesDeliveredClaim(t *testing.T){
	repo:=&workerRepo{claim:claimFixture()}
	w:=SecurityDeliveryWorker{Repository:repo,Sender:workerSender{idempotent:true},Policy:accessdomain.EnterpriseNotificationRetryPolicy(),WorkerID:"worker-185"}
	result,err:=w.RunOnce(context.Background())
	if err!=nil||!repo.completed||result.State!=accessdomain.NotificationStateDelivered{t.Fatalf("result=%+v completed=%v err=%v",result,repo.completed,err)}
}

func TestEnterprise185KnownTransientFailureRetries(t *testing.T){
	repo:=&workerRepo{claim:claimFixture()}
	w:=SecurityDeliveryWorker{Repository:repo,Sender:workerSender{err:workerFailure{retryable:true,known:true}},Policy:accessdomain.EnterpriseNotificationRetryPolicy(),WorkerID:"worker-185"}
	result,err:=w.RunOnce(context.Background())
	if err!=nil||repo.failedRetryable==nil||!*repo.failedRetryable||result.State!=accessdomain.NotificationStateRetryWait||repo.failureCode!="PROVIDER_REJECTED"{t.Fatalf("result=%+v retry=%v code=%s err=%v",result,repo.failedRetryable,repo.failureCode,err)}
}

func TestEnterprise185UnknownOutcomeRequiresIdempotentProvider(t *testing.T){
	for _,tc:=range []struct{name string;safe bool;want bool}{{"unsafe",false,false},{"idempotent",true,true}}{
		t.Run(tc.name,func(t *testing.T){
			repo:=&workerRepo{claim:claimFixture()}
			w:=SecurityDeliveryWorker{Repository:repo,Sender:workerSender{err:context.DeadlineExceeded,idempotent:tc.safe},Policy:accessdomain.EnterpriseNotificationRetryPolicy(),WorkerID:"worker-185"}
			result,err:=w.RunOnce(context.Background())
			if err!=nil||repo.failedRetryable==nil||*repo.failedRetryable!=tc.want{t.Fatalf("result=%+v retry=%v err=%v",result,repo.failedRetryable,err)}
			if tc.want&&result.State!=accessdomain.NotificationStateRetryWait{t.Fatalf("expected retry: %+v",result)}
			if !tc.want&&result.State!=accessdomain.NotificationStateManualReview{t.Fatalf("expected manual review: %+v",result)}
		})
	}
}

func TestEnterprise185UnclassifiedFailureOnlyRetriesForIdempotentProvider(t *testing.T){
	for _,tc:=range []struct{name string;safe bool;want bool}{{"unsafe",false,false},{"safe",true,true}}{
		t.Run(tc.name,func(t *testing.T){
			repo:=&workerRepo{claim:claimFixture()}
			w:=SecurityDeliveryWorker{Repository:repo,Sender:workerSender{err:errors.New("socket disappeared"),idempotent:tc.safe},Policy:accessdomain.EnterpriseNotificationRetryPolicy(),WorkerID:"worker-185"}
			result,err:=w.RunOnce(context.Background())
			if err!=nil||repo.failedRetryable==nil||*repo.failedRetryable!=tc.want{t.Fatalf("result=%+v retry=%v err=%v",result,repo.failedRetryable,err)}
		})
	}
}

var _ accessports.ReliableSecurityNotificationRepository = (*workerRepo)(nil)
var _ accessports.SecurityNotificationRetrySafety = workerSender{}
