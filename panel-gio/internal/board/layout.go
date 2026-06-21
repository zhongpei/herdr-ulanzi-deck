package board

import (
	"fmt"
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"github.com/herdr-deck/herdrdeck/displaymodel"
	"github.com/herdr-deck/herdrdeck/protocol"
)

// LayoutBoard renders the full Fleet Board.
// Pure Flex layout — each section is a Vertical Flex Rigid child.
// Background fills use the child's Min constraint to paint only the section area.
func LayoutBoard(gtx layout.Context, m displaymodel.Model, snap *protocol.FleetSnapshot, now time.Time, offline bool, phase float64, is *InputState, hidden map[string]bool) layout.Dimensions {
	// Window fill
	paint.FillShape(gtx.Ops, ColorBg, clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Op())

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// ── TopHealth ──
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(24))
			return section(gtx, ColorCardBg, func(gtx layout.Context) layout.Dimensions {
				return topHealthContent(gtx, m, now, offline, phase)
			})
		}),
		// ── Lens ──
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(44))
			return section(gtx, ColorCardBg, func(gtx layout.Context) layout.Dimensions {
				return lensContent(gtx, m, snap, hidden, is)
			})
		}),
		// ── Attention ──
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var cards []displaymodel.AgentCell
			// Priority: Blocked > Done > Working (sorted by builder), max 3
			for _, a := range m.Agents {
				if a.Status == protocol.StatusBlocked || a.Status == protocol.StatusDone || a.Status == protocol.StatusWorking {
					cards = append(cards, a)
					if len(cards) >= 3 {
						break
					}
				}
			}
			h := 60
			if len(cards) == 0 {
				h = 22
			}
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(h))
			return section(gtx, ColorBg, func(gtx layout.Context) layout.Dimensions {
				return attentionContent(gtx, m, cards, phase, is)
			})
		}),
		// ── Matrix ──
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Count unique machines for row height
			machSet := map[string]struct{}{}
			for _, a := range m.Agents {
				machSet[a.ConnName] = struct{}{}
			}
			h := 22 + len(machSet)*18
			if h > 300 {
				h = 300
			}
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(h))
			return section(gtx, ColorBg, func(gtx layout.Context) layout.Dimensions {
				return matrixContent(gtx, m, snap, phase)
			})
		}),
		// ── Selected ──
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(30))
			return section(gtx, ColorCardBg, func(gtx layout.Context) layout.Dimensions {
				return selectedContent(gtx, m, is)
			})
		}),
	)
}

// section paints a solid fill and then runs content on top.
// content sees constraints where Min = the fill size.
func section(gtx layout.Context, bg color.NRGBA, content func(layout.Context) layout.Dimensions) layout.Dimensions {
	fillSz := gtx.Constraints.Min
	paint.FillShape(gtx.Ops, bg, clip.Rect(image.Rectangle{Max: fillSz}).Op())

	dims := content(gtx)
	// Enforce minimum — ensure returned size is at least the fill area.
	if dims.Size.X < fillSz.X {
		dims.Size.X = fillSz.X
	}
	if dims.Size.Y < fillSz.Y {
		dims.Size.Y = fillSz.Y
	}
	return dims
}

// ─── Content helpers ──────────────────────────────────────

func topHealthContent(gtx layout.Context, m displaymodel.Model, now time.Time, offline bool, phase float64) layout.Dimensions {
	th := Theme
	stats := m.Stats.AgentStats
	total := stats.Blocked + stats.Working + stats.Idle + stats.Done + stats.Unknown
	timeStr := now.Format("15:04:05")
	liveLabel := fmt.Sprintf("● LIVE · %s", timeStr)
	liveColor := ColorTextSecondary
	if offline {
		liveLabel = "○ OFFLINE"
		liveColor = ColorTextDim
	}

	return layout.UniformInset(unit.Dp(3)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// Single row: stats on left, LIVE on right
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if total == 0 {
					l := material.Body1(th, "●0  waiting for fleet data...")
					l.Color = ColorTextSecondary
					l.TextSize = unit.Sp(8)
					return l.Layout(gtx)
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					statTotal(gtx, th, total),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Body1(th, "  │  ")
						l.Color = ColorTextDim
						l.TextSize = unit.Sp(8)
						return l.Layout(gtx)
					}),
					statLabel(gtx, th, stats.Blocked, "blocked", animIf(phase, stats.Blocked, ColorBlocked, PulseOpacity)),
					layout.Rigid(spacer4),
					statLabel(gtx, th, stats.Working, "working", animIf(phase, stats.Working, ColorWorking, BreathOpacity)),
					layout.Rigid(spacer4),
					statLabel(gtx, th, stats.Idle, "idle", ColorIdle),
					layout.Rigid(spacer4),
					statLabel(gtx, th, stats.Done, "done", animIf(phase, stats.Done, ColorDone, FlashOpacity)),
					layout.Rigid(spacer4),
					statLabel(gtx, th, stats.Unknown, "unknown", ColorUnknown),
				)
			}),
			layout.Flexed(1, layout.Spacer{Width: 0}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body1(th, liveLabel)
				l.Color = liveColor
				l.TextSize = unit.Sp(9)
				return l.Layout(gtx)
			}),
		)
	})
}

func lensContent(gtx layout.Context, m displaymodel.Model, snap *protocol.FleetSnapshot, hidden map[string]bool, is *InputState) layout.Dimensions {
	th := Theme
	return layout.UniformInset(unit.Dp(3)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					tagLabel(gtx, th, "FOCUS", 60),
					tagLabel(gtx, th, "MACHINE", 0),
					layout.Flexed(1, layout.Spacer{Width: 0}.Layout),
					tagLabel(gtx, th, "SPACES", 0),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if is != nil {
							return is.BtnActive.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								gtx.Constraints.Min.X = gtx.Dp(unit.Dp(60))
								l := material.Body1(th, m.NavAll.Label)
								l.Color = ColorAccent
								l.TextSize = unit.Sp(10)
								l.Font.Weight = font.Bold
								return l.Layout(gtx)
							})
						}
						gtx.Constraints.Min.X = gtx.Dp(unit.Dp(60))
						l := material.Body1(th, m.NavAll.Label)
						l.Color = ColorAccent
						l.TextSize = unit.Sp(10)
						l.Font.Weight = font.Bold
						return l.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, func() []layout.FlexChild {
							var children []layout.FlexChild
							if snap == nil || is == nil {
								return children
							}
							for _, m := range snap.Machines {
								m := m
								clk := is.MachineClicks[m.Name]
								if clk == nil {
									continue
								}
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return clk.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										isHidden := hidden[m.Name]
										l := material.Body1(th, m.Abbr)
										if isHidden {
											l.Color = ColorTextDim
										} else {
											l.Color = ColorAccent
										}
										l.TextSize = unit.Sp(9)
										l.Font.Weight = font.Bold
										return l.Layout(gtx)
									})
								}))
								children = append(children, layout.Rigid(layout.Spacer{Width: SpacingSmall}.Layout))
							}
							if len(children) > 0 {
								children = children[:len(children)-1]
							}
							return children
						}()...)
					}),
					layout.Flexed(1, layout.Spacer{Width: 0}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// SPACES: two-line vertical layout (matches FOCUS+MACHINE row height)
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, func() []layout.FlexChild {
							var rows []layout.FlexChild
							if is == nil || len(is.SpaceOrder) == 0 {
								return rows
							}
							// Split into two halves
							n := len(is.SpaceOrder)
							mid := (n + 1) / 2
							halves := [][]string{is.SpaceOrder[:mid], is.SpaceOrder[mid:]}
							for _, half := range halves {
								half := half
								rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, func() []layout.FlexChild {
										var chips []layout.FlexChild
										for _, ws := range half {
											ws := ws
											clk := is.SpaceClicks[ws]
											if clk == nil {
												continue
											}
											chips = append(chips, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return clk.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
													isHidden := is.HiddenSpaces[ws]
													l := material.Body1(th, ws)
													if isHidden {
														l.Color = ColorTextDim
													} else {
														l.Color = ColorAccent
													}
													l.TextSize = unit.Sp(9)
													l.Font.Weight = font.Bold
													return l.Layout(gtx)
												})
											}))
											chips = append(chips, layout.Rigid(layout.Spacer{Width: SpacingSmall}.Layout))
										}
										if len(chips) > 0 {
											chips = chips[:len(chips)-1]
										}
										return chips
									}()...)
								}))
							}
							return rows
						}()...)
					}),
				)
			}),
		)
	})
}

func tagLabel(gtx layout.Context, th *material.Theme, txt string, minW unit.Dp) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		if minW > 0 {
			gtx.Constraints.Min.X = gtx.Dp(minW)
		}
		l := material.Body1(th, txt)
		l.Color = ColorTextDim
		l.TextSize = unit.Sp(8)
		l.Font.Weight = font.Bold
		return l.Layout(gtx)
	})
}

func attentionContent(gtx layout.Context, m displaymodel.Model, cards []displaymodel.AgentCell, phase float64, is *InputState) layout.Dimensions {
	th := Theme
	return layout.UniformInset(SpacingSmall).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body1(th, "ATTENTION")
				l.Font.Weight = font.Bold
				l.Color = ColorTextPrimary
				l.TextSize = unit.Sp(9)
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(cards) == 0 {
					l := material.Body1(th, "all clear")
					l.Color = ColorTextSecondary
					l.TextSize = unit.Sp(9)
					return l.Layout(gtx)
				}
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, func() []layout.FlexChild {
					children := make([]layout.FlexChild, 0, len(cards))
					for i, c := range cards {
						c := c
						if is != nil {
							is.RegisterAgent(fmt.Sprintf("agent-%d", i), AgentRef{
								PaneID: c.PaneID, Machine: c.ConnName, Name: c.Name,
							})
						}
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return renderAgentCard(gtx, th, c, phase)
						}))
					}
					return children
				}()...)
			}),
		)
	})
}

func matrixContent(gtx layout.Context, m displaymodel.Model, snap *protocol.FleetSnapshot, phase float64) layout.Dimensions {
	th := Theme
	return layout.UniformInset(SpacingSmall).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body1(th, "AGENT GRID")
				l.Font.Weight = font.Bold
				l.Color = ColorTextPrimary
				l.TextSize = unit.Sp(9)
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(m.Agents) == 0 {
					l := material.Body1(th, "no agents")
					l.Color = ColorTextSecondary
					l.TextSize = unit.Sp(9)
					return l.Layout(gtx)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, func() []layout.FlexChild {
					// Group by machine
					type mg struct {
						name   string
						agents []displaymodel.AgentCell
					}
					var groups []mg
					mIdx := map[string]int{}
					for _, a := range m.Agents {
						if idx, ok := mIdx[a.ConnName]; ok {
							groups[idx].agents = append(groups[idx].agents, a)
						} else {
							mIdx[a.ConnName] = len(groups)
							groups = append(groups, mg{a.ConnName, []displaymodel.AgentCell{a}})
						}
					}
					children := make([]layout.FlexChild, 0, len(groups))
					for _, g := range groups {
						g := g
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Min.X = gtx.Dp(MinMachineCol)
									ml := material.Body1(th, g.name)
									ml.Color = ColorTextPrimary
									ml.TextSize = unit.Sp(9)
									ml.Font.Weight = font.Bold
									return ml.Layout(gtx)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, func() []layout.FlexChild {
										chips := make([]layout.FlexChild, 0, len(g.agents))
										for _, a := range g.agents {
											a := a
											chips = append(chips, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return renderChip(gtx, th, a, phase)
											}))
										}
										return chips
									}()...)
								}),
							)
						}))
						// Separator line between rows
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							paint.FillShape(gtx.Ops, ColorDivider, clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, 1)).Op())
							return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 1)}
						}))
					}
					// Remove trailing separator
					if len(children) > 0 {
						children = children[:len(children)-1]
					}
					return children
				}()...)
			}),
		)
	})
}

func selectedContent(gtx layout.Context, m displaymodel.Model, is *InputState) layout.Dimensions {
	th := Theme
	var line1, line2 string
	if len(m.Agents) > 0 {
		a := m.Agents[0]
		line1 = fmt.Sprintf("SELECTED  %s · %s · %s / %s", a.Name, a.Agent, a.ConnAbbr, truncate(a.WsLabel, 16))
		line2 = fmt.Sprintf("%s  |  %s", lowStatus(a.Status), ShortcutHint())
	} else {
		line1 = "no agents"
		line2 = ShortcutHint()
	}

	return layout.UniformInset(SpacingSmall).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body1(th, line1)
				l.Color = ColorTextSecondary
				l.TextSize = unit.Sp(9)
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						l := material.Body1(th, line2)
						l.Color = ColorTextDim
						l.TextSize = unit.Sp(7)
						return l.Layout(gtx)
					}),
					layout.Flexed(1, layout.Spacer{Width: 0}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if is != nil && is.HideWindow != nil {
							return is.HideBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								l := material.Body1(th, " _ ")
								l.Color = ColorTextDim
								l.TextSize = unit.Sp(9)
								return l.Layout(gtx)
							})
						}
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}),
				)
			}),
		)
	})
}

// ─── Sub-renderers ─────────────────────────────────────────

func renderAgentCard(gtx layout.Context, th *material.Theme, a displaymodel.AgentCell, phase float64) layout.Dimensions {
	sc := StatusColor(a.Status)
	ac := sc
	switch a.Status {
	case protocol.StatusWorking:
		ac = AnimateColor(sc, BreathOpacity(phase))
	case protocol.StatusBlocked:
		ac = AnimateColor(sc, PulseOpacity(phase))
	case protocol.StatusDone:
		ac = AnimateColor(sc, FlashOpacity(phase))
	}

	return layout.UniformInset(SpacingTiny).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Dp(MinCardWidth)
		paint.FillShape(gtx.Ops, ColorCardBg, clip.UniformRRect(
			image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(CornerRadius),
		).Op(gtx.Ops))
		return layout.UniformInset(SpacingSmall).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			// 4 rows: symbol+name / status / workspace / machine
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					sym := material.Body1(th, StatusSymbol(a.Status))
					sym.Color = ac
					sym.TextSize = unit.Sp(11)
					name := material.Body1(th, truncate(a.Name, 12))
					name.Color = ColorTextPrimary
					name.TextSize = unit.Sp(10)
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(sym.Layout),
						layout.Rigid(layout.Spacer{Width: SpacingSmall}.Layout),
						layout.Rigid(name.Layout),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					l := material.Body1(th, lowStatus(a.Status))
					l.Color = ac
					l.TextSize = unit.Sp(8)
					return l.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					ws := a.WsLabel
					if ws == "" {
						ws = "-"
					}
					l := material.Body1(th, truncate(ws, 12))
					l.Color = ColorTextSecondary
					l.TextSize = unit.Sp(8)
					return l.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					mach := a.ConnAbbr
					if mach == "" {
						mach = "?"
					}
					l := material.Body1(th, mach)
					l.Color = ColorTextDim
					l.TextSize = unit.Sp(8)
					return l.Layout(gtx)
				}),
			)
		})
	})
}

func renderChip(gtx layout.Context, th *material.Theme, a displaymodel.AgentCell, phase float64) layout.Dimensions {
	sc := StatusColor(a.Status)
	ac := sc
	switch a.Status {
	case protocol.StatusWorking:
		ac = AnimateColor(sc, BreathOpacity(phase))
	case protocol.StatusBlocked:
		ac = AnimateColor(sc, PulseOpacity(phase))
	case protocol.StatusDone:
		ac = AnimateColor(sc, FlashOpacity(phase))
	}
	chip := material.Body1(th, " "+StatusSymbol(a.Status)+" "+truncate(a.Name, 12)+" ")
	chip.Color = ac
	chip.TextSize = unit.Sp(8)

	paint.FillShape(gtx.Ops, ColorCardBg, clip.UniformRRect(
		image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(ChipRadius),
	).Op(gtx.Ops))
	return layout.UniformInset(SpacingTiny).Layout(gtx, chip.Layout)
}

// statTotal renders "total N" in blue.
func statTotal(gtx layout.Context, th *material.Theme, n int) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		l := material.Body1(th, fmt.Sprintf("TOTAL %d", n))
		l.Color = ColorTextPrimary
		l.TextSize = unit.Sp(8)
		l.Font.Weight = font.Bold
		return l.Layout(gtx)
	})
}

// statLabel renders "count name" with count in numColor and name in ColorTextDim.
func statLabel(gtx layout.Context, th *material.Theme, count int, name string, numColor color.NRGBA) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body1(th, fmt.Sprintf("%d", count))
				l.Color = numColor
				l.TextSize = unit.Sp(8)
				l.Font.Weight = font.Bold
				return l.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				l := material.Body1(th, " "+name)
				l.Color = ColorTextDim
				l.TextSize = unit.Sp(8)
				return l.Layout(gtx)
			}),
		)
	})
}

func spacer4(gtx layout.Context) layout.Dimensions {
	return layout.Dimensions{Size: image.Pt(gtx.Dp(unit.Dp(4)), 0)}
}

// animateColor applies a per-frame opacity animation to a base color.
func animateColor(phase float64, base color.NRGBA, fn func(float64) float64) color.NRGBA {
	return AnimateColor(base, fn(phase))
}

// animIf returns animated color only when count > 0, otherwise static base.
func animIf(phase float64, count int, base color.NRGBA, fn func(float64) float64) color.NRGBA {
	if count <= 0 {
		return base
	}
	return AnimateColor(base, fn(phase))
}

func lowStatus(s protocol.AgentStatus) string {
	switch s {
	case protocol.StatusBlocked:
		return "blocked"
	case protocol.StatusDone:
		return "done"
	case protocol.StatusWorking:
		return "working"
	case protocol.StatusIdle:
		return "idle"
	case protocol.StatusUnknown:
		return "unknown"
	default:
		return "?"
	}
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n-1]) + "…"
	}
	return s
}
