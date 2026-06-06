<script lang="ts">
	// Choropleth of ONE 参議院 記名投票 over 都道府県 selection districts (1:N; ADR-000010).
	// A prefecture has multiple senators, so a single fill can't be one member's vote.
	// To avoid a 賛同率 heatmap (rejected as §3/§5-C scoring), the fill is a FACTUAL
	// category — 全員賛成 / 全員反対 / 割れ / 記録なし — and the raw 内訳 (賛成n/反対m) is shown
	// in context on click (§7). The boundary GeoJSON is a static basemap, not data.
	import { onMount, onDestroy } from 'svelte';
	import type { PrefectureTally } from '$lib/types';

	let { prefectures = [] }: { prefectures?: PrefectureTally[] } = $props();

	let el: HTMLDivElement;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any
	let map: any;

	const COLORS = { yes: '#2e9e5b', no: '#d2454a', split: '#e0a838', none: '#d7d9dd' };

	// match ['get','id'] → factual category. id is the JIS prefecture code; a 合区 tally
	// ("31,32") is applied to both prefectures.
	function fillColorExpr(byCode: Map<number, PrefectureTally>): unknown {
		const groups: Record<'yes' | 'no' | 'split', number[]> = { yes: [], no: [], split: [] };
		for (const [code, t] of byCode) {
			const y = t.yes ?? 0;
			const n = t.no ?? 0;
			if (y > 0 && n === 0) groups.yes.push(code);
			else if (n > 0 && y === 0) groups.no.push(code);
			else if (y > 0 && n > 0) groups.split.push(code);
		}
		const expr: unknown[] = ['match', ['get', 'id']];
		for (const k of ['yes', 'no', 'split'] as const) {
			if (groups[k].length) expr.push(groups[k], COLORS[k]);
		}
		expr.push(COLORS.none);
		return expr.length > 3 ? expr : COLORS.none;
	}

	onMount(async () => {
		const byCode = new Map<number, PrefectureTally>();
		for (const t of prefectures) {
			for (const c of (t.districtCode ?? '').split(',')) {
				const code = Number(c);
				if (code) byCode.set(code, t);
			}
		}

		const maplibregl = (await import('maplibre-gl')).default;
		await import('maplibre-gl/dist/maplibre-gl.css');

		map = new maplibregl.Map({
			container: el,
			style: {
				version: 8,
				sources: {},
				layers: [{ id: 'bg', type: 'background', paint: { 'background-color': '#0e1014' } }]
			},
			center: [137.5, 38.2],
			zoom: 4,
			attributionControl: false,
			// One finger scrolls the page, two fingers pan/zoom (DESIGN_LANGUAGE §9.4).
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
				customAttribution: '境界: dataofjapan/land（都道府県）／簡略化: mapshaper'
			})
		);
		await new Promise<void>((resolve) => map.on('load', () => resolve()));

		map.addSource('pref', { type: 'geojson', data: '/geo/prefectures.geojson' });
		map.addLayer({
			id: 'fill',
			type: 'fill',
			source: 'pref',
			paint: { 'fill-color': fillColorExpr(byCode), 'fill-opacity': 0.82 }
		});
		map.addLayer({
			id: 'outline',
			type: 'line',
			source: 'pref',
			paint: { 'line-color': '#0e1014', 'line-width': 0.4 }
		});

		const popup = new maplibregl.Popup({ closeButton: false });
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		map.on('click', 'fill', (e: any) => {
			const f = e.features?.[0];
			if (!f) return;
			const code = f.properties.id as number;
			const name = (f.properties.nam_ja as string) ?? String(code);
			const t = byCode.get(code);
			const body = t
				? `賛成 ${t.yes ?? 0} ／ 反対 ${t.no ?? 0}${t.abstain ? ` ／ 棄権・欠席 ${t.abstain}` : ''}`
				: '記録なし';
			popup.setLngLat(e.lngLat).setHTML(`<strong>${name}</strong><br>${body}`).addTo(map);
		});
		map.on('mouseenter', 'fill', () => (map.getCanvas().style.cursor = 'pointer'));
		map.on('mouseleave', 'fill', () => (map.getCanvas().style.cursor = ''));
	});

	onDestroy(() => map?.remove());
</script>

<div class="map" bind:this={el} role="img" aria-label="都道府県別の参議院記名投票地図"></div>

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
