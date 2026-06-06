<script lang="ts">
	// Choropleth of ONE 記名投票 over the 衆 small-electoral-districts (ADR-000008).
	// Colour encodes a factual vote category only (賛成/反対/棄権/記録なし) — never an
	// aggregate score (§3/§5-C). The boundary GeoJSON is a static basemap (国土数値情報),
	// not observed data; the facts (district → option) come from the API with provenance.
	// Districts join the basemap by `kucode` (== ken*100+ku). 比例 members carry no
	// district and are shown by the page's companion panel, never erased (§5).
	import { onMount, onDestroy } from 'svelte';
	import type { Vote } from '$lib/types';
	import { VOTE_COLORS, MAP_BASE } from '$lib/voteColors';

	let { votes = [] }: { votes?: Vote[] } = $props();

	let el: HTMLDivElement;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let map: any;

	const OPT_JA: Record<string, string> = { yes: '賛成', no: '反対', abstain: '棄権' };

	// A MapLibre `match` expression colouring each district by its member's option.
	// Empty option groups are skipped (a match needs ≥1 label/output pair); when no
	// district has a record at all, fall back to a flat "no record" colour.
	function fillColorExpr(byDistrict: Map<number, Vote>): unknown {
		const groups: Record<string, number[]> = { yes: [], no: [], abstain: [] };
		for (const [kucode, v] of byDistrict) {
			const g = groups[v.option ?? ''];
			if (g) g.push(kucode);
		}
		const expr: unknown[] = ['match', ['get', 'kucode']];
		for (const opt of ['yes', 'no', 'abstain']) {
			if (groups[opt].length) expr.push(groups[opt], VOTE_COLORS[opt]);
		}
		expr.push(VOTE_COLORS.none); // default = 記録なし
		return expr.length > 3 ? expr : VOTE_COLORS.none;
	}

	onMount(async () => {
		// district_code (== GeoJSON kucode) → the sitting member's recorded vote.
		// Built here (page data is static), so it reads the props inside the closure.
		const byDistrict = new Map<number, Vote>();
		for (const v of votes) {
			if (!v.isPr && v.districtCode) byDistrict.set(Number(v.districtCode), v);
		}

		const maplibregl = (await import('maplibre-gl')).default;
		await import('maplibre-gl/dist/maplibre-gl.css');

		map = new maplibregl.Map({
			container: el,
			// Blank style — deliberately NO third-party basemap tiles (passive /
			// self-hosted ethos): the districts themselves are the map.
			style: {
				version: 8,
				sources: {},
				layers: [{ id: 'bg', type: 'background', paint: { 'background-color': MAP_BASE } }]
			},
			center: [137.5, 38.2],
			zoom: 4,
			attributionControl: false,
			// Don't trap page scroll on touch (DESIGN_LANGUAGE §9.4): one finger
			// scrolls the page, two fingers pan/zoom. Hint text localised to JA.
			cooperativeGestures: true,
			locale: {
				'CooperativeGesturesHandler.MobileHelpText': '2本指で地図を移動',
				'CooperativeGesturesHandler.WindowsHelpText': 'Ctrl + スクロールでズーム',
				'CooperativeGesturesHandler.MacHelpText': '⌘ + スクロールでズーム'
			}
		});
		map.addControl(new maplibregl.NavigationControl({ showCompass: false }), 'top-right');
		map.addControl(
			new maplibregl.AttributionControl({
				customAttribution: '境界: 国土数値情報（国土交通省）／加工: SmartNews Media Research Institute'
			})
		);

		await new Promise<void>((resolve) => map.on('load', () => resolve()));

		map.addSource('districts', {
			type: 'geojson',
			data: '/geo/senkyoku289.geojson',
			promoteId: 'kucode'
		});
		map.addLayer({
			id: 'fill',
			type: 'fill',
			source: 'districts',
			paint: { 'fill-color': fillColorExpr(byDistrict), 'fill-opacity': 0.82 }
		});
		map.addLayer({
			id: 'outline',
			type: 'line',
			source: 'districts',
			paint: { 'line-color': MAP_BASE, 'line-width': 0.4 }
		});

		const popup = new maplibregl.Popup({ closeButton: false });
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		map.on('click', 'fill', (e: any) => {
			const f = e.features?.[0];
			if (!f) return;
			const kucode = f.properties.kucode as number;
			const v = byDistrict.get(kucode);
			const name = (f.properties.kuname as string) ?? String(kucode);
			const opt = v ? (OPT_JA[v.option ?? ''] ?? '—') : '記録なし';
			const who = v?.voterName ? `${v.voterName}${v.parliamentaryGroup ? `（${v.parliamentaryGroup}）` : ''}` : '';
			popup
				.setLngLat(e.lngLat)
				.setHTML(`<strong>${name}</strong>${who ? `<br>${who}` : ''}<br>投票: ${opt}`)
				.addTo(map);
		});
		map.on('mouseenter', 'fill', () => (map.getCanvas().style.cursor = 'pointer'));
		map.on('mouseleave', 'fill', () => (map.getCanvas().style.cursor = ''));
	});

	onDestroy(() => map?.remove());
</script>

<div class="map" bind:this={el} role="img" aria-label="選挙区別の記名投票地図"></div>

<style>
	.map {
		width: 100%;
		/* Shorter on phones so the map never fills the viewport (§9.4). */
		height: clamp(360px, 60vh, 520px);
		border-radius: 8px;
		overflow: hidden;
		border: 1px solid var(--hairline-2);
	}
	/* Popup themed to the dark token surface (§9.4) — was a light card before. */
	:global(.maplibregl-popup-content) {
		background: var(--surface-3);
		color: var(--text-1);
		border: 1px solid var(--hairline-2);
		border-radius: var(--r-sm);
		font-size: 13px;
		line-height: 1.5;
	}
	:global(.maplibregl-popup-tip) {
		display: none;
	}
</style>
