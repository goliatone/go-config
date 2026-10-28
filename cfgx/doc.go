// Package cfgx builds typed configuration from loose input.
//
// The helpers defined here are intentionally decoupled from config.Container so other
// packages can decode, preprocess, and validate their own config structs without
// depending on application-level wiring. See the repository README for usage.
//
// Option catalog:
//   - Defaults: WithDefaults, WithDefaultFunc.
//   - Preprocessing: WithPreprocess, WithPreprocessFunc, WithPreprocessEvalFuncs, WithMerge,
//     WithoutDefaultHooks/WithDefaultHooks.
//   - Decoder behavior: WithDecoder, WithDecoderConfig, WithDecodeHooks, WithStrictKeys,
//     WithWeakTyping, WithTagName.
//   - Validation: WithValidator, WithValidatorFunc.
//   - Ownership: WithOpaqueValues, WithDefaultsCloner, CloneDefaults, ValidateOpaque.
//   - Diagnostics: WithOptionError lets wrappers surface invalid option state.
//
// Hook helpers:
//   - OptionalBoolHook requires registration via RegisterOptionalBoolType so cfgx can manipulate
//     custom optional-bool implementations (config.OptionalBool registers itself during init).
//   - DurationHook mirrors mapstructure's string-to-duration helper.
//   - TextUnmarshalerHook preserves compatibility with encoding.Text(Un)Marshaler types.
package cfgx
