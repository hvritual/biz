package subscription
import("reflect";"testing")
func TestCE08OrderingIsTotalAcrossConcreteScopes(t *testing.T){
 rules:=[]Rule{{RuleID:"b",Priority:10,SalesScope:"domestic"},{RuleID:"a",Priority:10,SalesScope:"overseas"},{RuleID:"fallback",Priority:10,SalesScope:"*"}}
 for _,order:=range [][]int{{0,1,2},{0,2,1},{1,0,2},{1,2,0},{2,0,1},{2,1,0}}{
  input:=[]Rule{rules[order[0]],rules[order[1]],rules[order[2]]};before:=append([]Rule(nil),input...);got:=Ordered(input)
  if got[0].RuleID!="a"||got[1].RuleID!="b"||got[2].RuleID!="fallback"{t.Fatalf("non-total order: %+v",got)}
  if !reflect.DeepEqual(input,before){t.Fatal("sort changed source rules")}
 }
}
