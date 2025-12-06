//go:build windows
// +build windows

package engine

func (e *Engine) AICreateParkedATCAircraft(szContainerTitle string, szTailNumber string, szAirportID string, RequestID uint32) error {
	return e.api.AICreateParkedATCAircraft(szContainerTitle, szTailNumber, szAirportID, RequestID)
}
