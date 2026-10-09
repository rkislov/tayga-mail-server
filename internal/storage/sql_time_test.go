package storage
import("testing";"time")
func TestSQLTimeLegacyIMAPOffset(t *testing.T){
 var got time.Time
 for _,v:=range []any{"2020-02-27 14:04:40 +0500 +0500",[]byte("2020-02-27 14:04:40 +0500 +0500"),time.Date(2020,2,27,9,4,40,0,time.UTC),"2020-02-27T09:04:40Z"}{
  if err:=(sqlTime{&got}).Scan(v);err!=nil{t.Fatal(err)}
  if !got.Equal(time.Date(2020,2,27,9,4,40,0,time.UTC)){t.Fatalf("wrong instant %v",got)}
 }
}
