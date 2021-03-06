package rfc5424

import (
	"log"
	"os"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultSeverity         = Info
	defaultFacility         = Local0
	defaultStructuredDataID = "0@local"
)

var (
	defaultHostname = func() string {
		h, err := os.Hostname()
		if err != nil {
			panic(err)
		}
		return h
	}()

	defaultAppName = func() string {
		return path.Base(os.Args[0])
	}()

	defaultProcessID = func() string {
		return strconv.FormatInt(int64(os.Getpid()), 10)
	}()
)

type reflection struct {
	Type                           reflect.Type
	SeverityFieldIndex             int
	SeverityDefault                Severity
	FacilityFieldIndex             int
	FacilityDefault                Facility
	TimestampFieldIndex            int
	HostnameFieldIndex             int
	AppNameFieldIndex              int
	AppNameDefault                 string
	ProcessIDFieldIndex            int
	MessageIDFieldIndex            int
	MessageIDDefault               string
	MessageFieldIndex              int
	SDIDDefault                    string
	StructuredDataFieldReflections []structuredDataFieldReflection
}

type structuredDataFieldReflection struct {
	FieldIndex int
	OmitEmpty  bool
	FieldName  string
	SdID       string
}

func (r *reflection) GetStructuredDataFieldReflection(
	SdID string, FieldName string) *structuredDataFieldReflection {
	for _, fieldReflection := range r.StructuredDataFieldReflections {
		if fieldReflection.SdID == SdID && fieldReflection.FieldName == FieldName {
			return &fieldReflection
		}
	}
	return nil
}

var reflectionCache = map[string][]*reflection{}

func Reflect(t reflect.Type) *reflection {
	reflectionList, ok := reflectionCache[t.Name()]
	if !ok {
		r := reflectImpl(t)
		reflectionCache[t.Name()] = []*reflection{r}
		return r
	}

	for _, r := range reflectionList {
		if r.Type == t {
			return r
		}
	}
	r := reflectImpl(t)
	reflectionCache[t.Name()] = append(reflectionList, r)
	return r
}

var sdRegexp = regexp.MustCompile("^(\\d+@\\S+)( (.*))?$")

func reflectImpl(t reflect.Type) *reflection {
	r := reflection{
		Type:                           t,
		SeverityFieldIndex:             -1,
		SeverityDefault:                defaultSeverity,
		FacilityFieldIndex:             -1,
		FacilityDefault:                defaultFacility,
		TimestampFieldIndex:            -1,
		HostnameFieldIndex:             -1,
		AppNameFieldIndex:              -1,
		AppNameDefault:                 defaultAppName,
		ProcessIDFieldIndex:            -1,
		MessageIDFieldIndex:            -1,
		MessageIDDefault:               t.Name(),
		MessageFieldIndex:              -1,
		StructuredDataFieldReflections: []structuredDataFieldReflection{},
	}

	for fieldIndex := 0; fieldIndex < t.NumField(); fieldIndex++ {
		field := t.Field(fieldIndex)
		fieldTag := field.Tag.Get("log")
		switch field.Name {
		case "Severity":
			r.SeverityFieldIndex = fieldIndex
			if fieldTag != "" {
				severity, ok := severityNames[fieldTag]
				if !ok {
					panic("invalid tag on Severity field")
				}
				r.SeverityDefault = severity
			}
		case "Facility":
			r.FacilityFieldIndex = fieldIndex
			if fieldTag != "" {
				facility, ok := facilityNames[fieldTag]
				if !ok {
					panic("invalid tag on Facility field")
				}
				r.FacilityDefault = facility
			}
		case "Timestamp":
			r.TimestampFieldIndex = fieldIndex
		case "Hostname":
			r.HostnameFieldIndex = fieldIndex
		case "AppName":
			r.AppNameFieldIndex = fieldIndex
			if fieldTag != "" {
				r.AppNameDefault = fieldTag
			}
		case "ProcessID":
			r.ProcessIDFieldIndex = fieldIndex
		case "MessageID":
			r.MessageIDFieldIndex = fieldIndex
			if fieldTag != "" {
				r.MessageIDDefault = fieldTag
			}
		case "SDID":
			if fieldTag != "" {
				r.SDIDDefault = fieldTag
			}
		case "Message":
			r.MessageFieldIndex = fieldIndex
		default:
			// Message or Structured Data fields

			// if the field is private and not tagged, ignore it
			if fieldTag == "" && field.PkgPath != "" {
				// Field is not exported, skip it
				continue
			}

			tagParts := strings.Split(fieldTag, ",")

			// If the field is marked message it contains the message
			if len(tagParts) > 1 && tagParts[0] == "" && tagParts[1] == "message" {
				r.MessageFieldIndex = fieldIndex
				continue
			}

			fieldReflection := structuredDataFieldReflection{}
			fieldReflection.FieldIndex = fieldIndex
			fieldReflection.FieldName = tagParts[0]
			if r.SDIDDefault != "" {
				fieldReflection.SdID = r.SDIDDefault
			} else {
				fieldReflection.SdID = defaultStructuredDataID
			}

			matches := sdRegexp.FindAllStringSubmatch(fieldReflection.FieldName, -1)
			if matches != nil {
				fieldReflection.SdID = matches[0][1]
				fieldReflection.FieldName = matches[0][3]
			}

			if fieldReflection.FieldName == "" {
				// Generate a field name by converting the first letter to lowercase
				fieldReflection.FieldName = strings.ToLower(field.Name[0:1]) + field.Name[1:]
			}

			if len(tagParts) > 1 {
				for _, tagAttr := range tagParts[1:] {
					switch tagParts[1] {
					case "omitempty":
						fieldReflection.OmitEmpty = true
					default:
						log.Panicf("unknown tag %s on field %s of %s",
							tagAttr, field.Name, t.Name())
					}
				}
			}

			r.StructuredDataFieldReflections = append(r.StructuredDataFieldReflections,
				fieldReflection)
		}
	}
	return &r
}
