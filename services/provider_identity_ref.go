package services

import (
	"fmt"
	"strings"
)

func providerRefFromNumericID(providerID int64, providerName string) string {
	if providerID > 0 {
		return fmt.Sprintf("%d", providerID)
	}
	return strings.TrimSpace(providerName)
}

func providerRefFromStringID(providerID, providerName string) string {
	providerID = strings.TrimSpace(providerID)
	if providerID != "" {
		return providerID
	}
	return strings.TrimSpace(providerName)
}
