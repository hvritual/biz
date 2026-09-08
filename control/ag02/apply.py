from pathlib import Path
import runpy,subprocess,os,json
control=Path(__file__).resolve().parent
assert subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()=='7d5afbe9cb4b849be462d9a7aed65877ed227700'
assert not subprocess.check_output(['git','status','--porcelain'],text=True).strip()
test=Path('integration/ag02_owner_snapshot_mysql_test.go');assert not test.exists()
fixture=(control/'owner_snapshot_test.go.txt').read_text()
test.write_text(fixture)
subprocess.run(['gofmt','-w',str(test)],check=True)
fixture=test.read_text()
try:
 env=dict(os.environ,GOTOOLCHAIN='local',YUNKA_TEST_MYSQL_DSN='root:root@tcp(127.0.0.1:3306)/biz_ag02?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true')
 result=subprocess.run(['go','test','-json','-timeout=3m','-count=1','-tags=integration','./integration','-run','^TestAG02OwnerInvariantReadsCurrentStateAfterSnapshot$'],env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=240)
 evidence=Path(os.environ['RUNNER_TEMP'])/'ag02-evidence'
 (evidence/'owner-baseline-red.jsonl').write_bytes(result.stdout)
 events=[json.loads(s) for s in result.stdout.decode().splitlines()]
 failed={e.get('Test') for e in events if e.get('Action')=='fail' and 'Test' in e}
 root='TestAG02OwnerInvariantReadsCurrentStateAfterSnapshot'
 assert result.returncode==1 and failed=={root,root+'/revoke',root+'/suspend'}, 'not the expected two-schedule baseline failure'
 assert not any(e.get('Action')=='skip' for e in events)
 output=''.join(e.get('Output','') for e in events)
 for first in ['revoke','suspend']:
  assert 'AG02_STALE_OWNER_SNAPSHOT: first='+first+' got=<nil> want ErrLastTenantOwner' in output
 print('BASELINE_RED_CONFIRMED: unchanged main accepts both stale-snapshot invariant violations',flush=True)
finally:
 test.unlink()
assert not subprocess.check_output(['git','status','--porcelain'],text=True).strip()
runpy.run_path(str(control/'apply_encapsulation.py'),run_name='__main__')
test.write_text(fixture)
p=Path('internal/access/infrastructure/persistence/role.go');s=p.read_text()
old='''var activeOwners int64
		if err := db.Table("biz_member_roles mr").
			Joins("JOIN biz_memberships m ON m.tenant_id = mr.tenant_id AND m.user_id = mr.user_id AND m.status = ?", domain.TenantMemberStatusActive).
			Where("mr.tenant_id = ? AND mr.role_id = ?", tenantID, roleID).
			Count(&activeOwners).Error; err != nil {'''
new='''activeOwners, err := currentActiveOwnerCount(db, tenantID, roleID)
		if err != nil {''';assert old in s;s=s.replace(old,new,1)
old='''var activeOwners int64
	if err := db.Table("biz_member_roles mr").
		Joins("JOIN biz_memberships m ON m.tenant_id = mr.tenant_id AND m.user_id = mr.user_id AND m.status = ?", domain.TenantMemberStatusActive).
		Where("mr.tenant_id = ? AND mr.role_id = ?", tenantID, owner.ID).
		Count(&activeOwners).Error; err != nil {'''
new='''activeOwners, err := currentActiveOwnerCount(db, tenantID, owner.ID)
	if err != nil {''';assert old in s;s=s.replace(old,new,1)
needle='func (repository *TenantRoleRepository) roleFromRecord'
helper='''// currentActiveOwnerCount is called only after the owner-role row is locked in
// the caller's root UoW. A plain COUNT can retain an earlier REPEATABLE READ
// snapshot even after that lock is acquired. Read the actual joined rows with
// FOR UPDATE so a previous owner's committed removal/deactivation is visible.
// The common role lock serializes both mutation paths; transaction ownership
// and the process-wide isolation level remain with the existing runtime.
func currentActiveOwnerCount(db *gorm.DB, tenantID, roleID string) (int, error) {
	var owners []struct { UserID string }
	err := db.Table("biz_member_roles mr").
		Select("mr.user_id").
		Joins("JOIN biz_memberships m ON m.tenant_id = mr.tenant_id AND m.user_id = mr.user_id AND m.status = ?", domain.TenantMemberStatusActive).
		Where("mr.tenant_id = ? AND mr.role_id = ?", tenantID, roleID).
		Order("mr.user_id").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Find(&owners).Error
	return len(owners), err
}

'''
assert needle in s;s=s.replace(needle,helper+needle,1);p.write_text(s)
p=Path('docs/architecture/AG-02-tenant-encapsulation.md')
p.write_text(p.read_text()+'''
## Qualification blocker: owner invariant snapshot (Biz #18)

Full qualification exposed the existing cross-path owner race in
`TestB126RoleRevokeAndMemberSuspendCannotRemoveAllEffectiveOwners`. Both requests
could succeed because ordinary owner-count reads could retain an older REPEATABLE
READ snapshot after the common owner-role lock was acquired. The deterministic
`TestAG02OwnerInvariantReadsCurrentStateAfterSnapshot` preserves both transaction
orders and requires `ErrLastTenantOwner` from the second mutation.

The consumer repository now reads the joined active-owner rows with `FOR UPDATE`
under that same role lock and caller-owned transaction. This is an additional
consumer correctness repair, not a change to the seven moved TenantLifecycle
methods, framework transaction ownership or global isolation level. The original
concurrent test remains mandatory. Exact RED/GREEN and final disposition belong
to issue #18 and the qualification receipt; failures are not reclassified as passes.
''')
subprocess.run(['gofmt','-w',str(test),'internal/access/infrastructure/persistence/role.go'],check=True)
subprocess.run(['git','add',str(test),'internal/access/infrastructure/persistence/role.go','docs/architecture/AG-02-tenant-encapsulation.md'],check=True)
assert subprocess.check_output(['git','write-tree'],text=True).strip()=='a8c72fd6cb63b0692e8c2c363239c1ae98ef6750'
subprocess.run(['git','diff','--cached','--check'],check=True)
