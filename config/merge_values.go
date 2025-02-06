package config

// func MergeDefaultValues(def map[string]any) error {

// }

func MergeIgnoringNullValues(src, dest map[string]any) error {
	for k, v := range src {
		dv, ok := dest[k]
		if !ok {
			dest[k] = v
			continue
		}

		if v == nil {
			continue
		}

		var overwrite bool
		switch vv := v.(type) {
		case string:
			if vv != "" {
				overwrite = true
			}
		case []any:
			if len(vv) > 0 {
				overwrite = true
			}
		case map[string]any:
			if dvv, ok := dv.(map[string]any); ok {
				if err := MergeIgnoringNullValues(vv, dvv); err != nil {
					return err
				}
				continue
			}
			overwrite = true
		default:
			overwrite = true
		}

		if overwrite {
			dest[k] = v
		}
	}

	return nil
}
