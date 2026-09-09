package persistence
import("context";"sync")
// MemorySnapshotCache is bounded and owns copies. Disabling it changes latency,
// never authorization: the database stamp is checked before every cache lookup.
type MemorySnapshotCache struct{mu sync.Mutex;max int;values map[string][]byte;order []string}
func NewMemorySnapshotCache(max int)*MemorySnapshotCache{if max<1{max=1};return &MemorySnapshotCache{max:max,values:map[string][]byte{}}}
func(c *MemorySnapshotCache)Get(_ context.Context,key string)([]byte,error){c.mu.Lock();defer c.mu.Unlock();return append([]byte(nil),c.values[key]...),nil}
func(c *MemorySnapshotCache)Put(_ context.Context,key string,b []byte)error{c.mu.Lock();defer c.mu.Unlock();if _,ok:=c.values[key];!ok{if len(c.order)>=c.max{delete(c.values,c.order[0]);c.order=c.order[1:]};c.order=append(c.order,key)};c.values[key]=append([]byte(nil),b...);return nil}
