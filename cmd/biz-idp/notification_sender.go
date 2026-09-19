//go:build !qualification

package main

import "github.com/hvritual/biz/internal/access/ports"

func qualificationNotificationSender() ports.SecurityNotificationSender {
	return nil
}
