package provider

import "fmt"

func cloneMapStringAny(src map[string]interface{}) map[string]interface{} {
	copyMap := make(map[string]interface{}, len(src))
	for k, v := range src {
		copyMap[k] = v
	}
	return copyMap
}

func redactMapKeys(m map[string]interface{}, keys ...string) {
	for _, key := range keys {
		if _, present := m[key]; present {
			m[key] = "***REDACTED***"
		}
	}
}

func redactedCollectionForLog(items []interface{}, redact func(map[string]interface{})) string {
	redacted := make([]interface{}, 0, len(items))
	for _, entry := range items {
		m, ok := entry.(map[string]interface{})
		if !ok {
			redacted = append(redacted, entry)
			continue
		}

		copyMap := cloneMapStringAny(m)
		redact(copyMap)
		redacted = append(redacted, copyMap)
	}

	return fmt.Sprintf("%+v", redacted)
}
