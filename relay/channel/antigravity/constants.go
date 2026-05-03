package antigravity

const (
	ChannelName = "antigravity"
)

type antigravityRequestStyle string

const (
	antigravityRequestStyleNative    antigravityRequestStyle = "antigravity"
	antigravityRequestStyleGeminiCLI antigravityRequestStyle = "gemini-cli"

	antigravityGeminiCLIUserAgent      = "google-api-nodejs-client/9.15.1"
	antigravityGeminiCLIApiClient      = "gl-node/22.17.0"
	antigravityGeminiCLIClientMetadata = "ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI"
)

var ModelList = []string{
	"antigravity-gemini-3-pro",
	"antigravity-gemini-3.1-pro",
	"antigravity-gemini-3-flash",
	"antigravity-claude-sonnet-4-6",
	"antigravity-claude-opus-4-6-thinking",
	"gpt-5.4",
	"gpt-5.4-mini",
	"gpt-5.4-openai-compact",
	"gpt-5.4-mini-openai-compact",
	"gpt-5.5",
	"gpt-5.5-mini",
	"gpt-5.5-openai-compact",
	"gpt-5.5-mini-openai-compact",
}

var modelAliasToUpstream = map[string]string{
	"antigravity-gemini-3-pro":             "gemini-3-pro-low",
	"antigravity-gemini-3.1-pro":           "gemini-3.1-pro-low",
	"antigravity-gemini-3-flash":           "gemini-3-flash",
	"antigravity-claude-sonnet-4-6":        "claude-sonnet-4-6",
	"antigravity-claude-opus-4-6-thinking": "claude-opus-4-6-thinking",
	"gpt-5.4":                              "gemini-3-flash",
	"gpt-5.4-mini":                         "gemini-3-flash",
	"gpt-5.4-openai-compact":               "gemini-3-flash",
	"gpt-5.4-mini-openai-compact":          "gemini-3-flash",
	"gpt-5.5":                              "gemini-3.1-pro-low",
	"gpt-5.5-mini":                         "gemini-3-flash",
	"gpt-5.5-openai-compact":               "gemini-3.1-pro-low",
	"gpt-5.5-mini-openai-compact":          "gemini-3-flash",
}

var responsesModelAliasToUpstream = map[string]string{
	"gpt-5.4":                     "gemini-3-flash-preview",
	"gpt-5.4-mini":                "gemini-3-flash-preview",
	"gpt-5.4-openai-compact":      "gemini-3-flash-preview",
	"gpt-5.4-mini-openai-compact": "gemini-3-flash-preview",
}
