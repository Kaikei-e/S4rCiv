// Vote-category colours — a FACTUAL category palette, never a value scale
// (DESIGN_LANGUAGE §6/§9.4, ADR-000020). 賛成=green / 反対=red is forbidden: green implies
// approval-is-good, red implies rejection-is-bad — a value judgment the non-partisan stance
// rules out. These are the neutral data-viz hues (dv-1 青 / dv-2 琥珀 / dv-4 紫) plus a muted
// grey for "no record". Literal hex (not CSS vars) because MapLibre paint expressions need
// concrete colours; the page legends import the SAME constants so legend and map never drift.
// Dark-tuned — the choropleth is effectively dark regardless of theme.
//
// 棄権 (衆 district map) and 割れ (参 prefecture map) are the "third / mixed" category and
// never appear on the same map, so both take dv-4.
export const VOTE_COLORS: Record<string, string> = {
	yes: '#5e97c9', // dv-1 青 — 賛成 / 全員賛成
	no: '#d2a24f', // dv-2 琥珀 — 反対 / 全員反対
	abstain: '#b487c9', // dv-4 紫 — 棄権
	split: '#b487c9', // dv-4 紫 — 割れ（賛否混在）
	none: '#5a6473' // 中立グレー（--text-faint dark）— 記録なし
};

// Blank-style background + district outlines, matched to --canvas (dark). The map ships no
// third-party basemap tiles (passive / self-hosted ethos) — the shapes are the map.
export const MAP_BASE = '#0a0d12';
