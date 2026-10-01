package envvar

// 包内私有的纯工具（与业务无关的字符串 / map 处理，无 IO、无全局状态）。
// 仅 envvar 使用；若后续其它包需要，再提升到 pkg/tools。

func splitAndTrim(s, sep string) []string {
	if s == "" {
		return []string{}
	}
	parts := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if len(sep) > 0 && i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			part := trimSpaces(s[start:i])
			if part != "" {
				parts = append(parts, part)
			}
			i += len(sep) - 1
			start = i + 1
		}
	}
	if start < len(s) {
		part := trimSpaces(s[start:])
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func trimSpaces(s string) string {
	left := 0
	right := len(s)
	for left < right && (s[left] == ' ' || s[left] == '\t' || s[left] == '\r' || s[left] == '\n') {
		left++
	}
	for right > left && (s[right-1] == ' ' || s[right-1] == '\t' || s[right-1] == '\r' || s[right-1] == '\n') {
		right--
	}
	return s[left:right]
}

func joinNonEmpty(items []string, sep string) string {
	nonEmpty := make([]string, 0, len(items))
	for _, item := range items {
		if item != "" {
			nonEmpty = append(nonEmpty, item)
		}
	}
	if len(nonEmpty) == 0 {
		return ""
	}
	result := nonEmpty[0]
	for i := 1; i < len(nonEmpty); i++ {
		result += sep + nonEmpty[i]
	}
	return result
}

func dedupedCopy(items []string) []string {
	if items == nil {
		return []string{}
	}
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func cloneMap(m map[string]string) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
