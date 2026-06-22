package render

// Agent brand colors (hex strings for SVG fill).
var AgentColors = map[string]string{
	"pi":          "#7C3AED", // purple
	"claude":      "#D97757", // warm brown
	"cursor":      "#00B884", // teal
	"cline":       "#2563EB", // blue
	"codex":       "#1E293B", // dark slate
	"gemini":      "#4285F4", // google blue
	"copilot":     "#8957E5", // purple
	"devin":       "#FF6B35", // orange
	"grok":        "#1DA1F2", // twitter blue
	"kimi":        "#FF6B6B", // coral
	"kilo":        "#10B981", // emerald
	"kiro":        "#F59E0B", // amber
	"opencode":    "#6366F1", // indigo
	"qodercli":    "#8B5CF6", // violet
	"amp":         "#EC4899", // pink
	"antigravity": "#06B6D4", // cyan
	"droid":       "#84CC16", // lime
	"hermes":      "#F97316", // orange
	"unknown":     "#6B7280", // gray
}

// Status indicator colors.
var StatusColors = map[string]string{
	"done":    "#27AE60",
	"idle":    "#7F8C8D",
	"working": "#F39C12",
	"blocked": "#E74C3C",
	"unknown": "#95A5A6",
}
