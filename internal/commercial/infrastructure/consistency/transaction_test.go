package consistency

import (
	"errors"
	"fmt"
	mysql "github.com/go-sql-driver/mysql"
	"testing"
)

func TestCE06TransientClassification(t *testing.T) {
	for _, n := range []uint16{1213, 1205, 1062, 1049} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			err := fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: n})
			if Transient(err) != (n == 1213 || n == 1205) {
				t.Fatal(n)
			}
		})
	}
	if Transient(errors.New("1213 deadlock")) || Transient(nil) {
		t.Fatal("must use typed errors")
	}
}
