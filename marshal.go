package rfc5424

import (
	"bytes"
	"fmt"
	"time"
	"unicode/utf8"
)

// allowLongSdNames is true to allow names longer than the RFC-specified limit
// of 32-characters. (When true, this violates RFC-5424).
const allowLongSdNames = true

type errorInvalidValue struct {
	Property string
	Value    interface{}
}

func (e errorInvalidValue) Error() string {
	return fmt.Sprintf("Message cannot be serialized because %s is invalid: %v",
		e.Property, e.Value)
}

// InvalidValue returns an invalid value error with the given property
func InvalidValue(property string, value interface{}) error {
	return errorInvalidValue{Property: property, Value: value}
}

func nilify(x string) string {
	if x == "" {
		return "-"
	}
	return x
}

func escapeSDParam(s string) string {
	escapeCount := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '"', ']':
			escapeCount++
		}
	}
	if escapeCount == 0 {
		return s
	}

	t := make([]byte, len(s)+escapeCount)
	j := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\', '"', ']':
			t[j] = '\\'
			t[j+1] = c
			j += 2
		default:
			t[j] = s[i]
			j++
		}
	}
	return string(t)
}

func isPrintableUsASCII(s string) bool {
	for _, ch := range s {
		if ch < 33 || ch > 126 {
			return false
		}
	}
	return true
}

func isValidSdName(s string) bool {
	if !allowLongSdNames && len(s) > 32 {
		return false
	}
	for _, ch := range s {
		if ch < 33 || ch > 126 {
			return false
		}
		if ch == '=' || ch == ']' || ch == '"' {
			return false
		}
	}
	return true
}

func (m Message) assertValid() error {
	if m.Severity < 0 || m.Severity > 8 {
		return InvalidValue("Severity", m.Severity)
	}
	if m.Facility < 0 || m.Facility > 23 {
		return InvalidValue("Facility", m.Facility)
	}

	// HOSTNAME        = NILVALUE / 1*255PRINTUSASCII
	if !isPrintableUsASCII(m.Hostname) {
		return InvalidValue("Hostname", m.Hostname)
	}
	if len(m.Hostname) > 255 {
		return InvalidValue("Hostname", m.Hostname)
	}

	// APP-NAME        = NILVALUE / 1*48PRINTUSASCII
	if !isPrintableUsASCII(m.AppName) {
		return InvalidValue("AppName", m.AppName)
	}
	if len(m.AppName) > 48 {
		return InvalidValue("AppName", m.AppName)
	}

	// PROCID          = NILVALUE / 1*128PRINTUSASCII
	if !isPrintableUsASCII(m.ProcessID) {
		return InvalidValue("ProcessID", m.ProcessID)
	}
	if len(m.ProcessID) > 128 {
		return InvalidValue("ProcessID", m.ProcessID)
	}

	// MSGID           = NILVALUE / 1*32PRINTUSASCII
	if !isPrintableUsASCII(m.MessageID) {
		return InvalidValue("MessageID", m.MessageID)
	}
	if len(m.MessageID) > 32 {
		return InvalidValue("MessageID", m.MessageID)
	}

	for _, sdElement := range m.StructuredData {
		if !isValidSdName(sdElement.ID) {
			return InvalidValue("StructuredData/ID", sdElement.ID)
		}
		for _, sdParam := range sdElement.Parameters {
			if !isValidSdName(sdParam.Name) {
				return InvalidValue("StructuredData/Name", sdParam.Name)
			}
			if !utf8.ValidString(sdParam.Value) {
				return InvalidValue("StructuredData/Value", sdParam.Value)
			}
		}
	}
	return nil
}

// MarshalBinary marshals the message to a byte slice, or returns an error
func (m Message) MarshalBinary() ([]byte, error) {
	if err := m.assertValid(); err != nil {
		return nil, err
	}

	b := bytes.NewBuffer(nil)
	fmt.Fprintf(b, "<%d>1 %s %s %s %s %s ",
		m.Severity|m.Facility<<3,
		m.Timestamp.Format(time.RFC3339Nano),
		nilify(m.Hostname),
		nilify(m.AppName),
		nilify(m.ProcessID),
		nilify(m.MessageID))

	if len(m.StructuredData) == 0 {
		fmt.Fprint(b, "-")
	}
	for _, sdElement := range m.StructuredData {
		fmt.Fprintf(b, "[%s", sdElement.ID)
		for _, sdParam := range sdElement.Parameters {
			fmt.Fprintf(b, " %s=\"%s\"", sdParam.Name,
				escapeSDParam(sdParam.Value))
		}
		fmt.Fprintf(b, "]")
	}

	if len(m.Message) > 0 {
		fmt.Fprint(b, " ")
		b.Write(m.Message)
	}
	return b.Bytes(), nil
}
