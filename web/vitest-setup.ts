// Registers @testing-library/jest-dom matchers (toBeInTheDocument, toHaveAttribute,
// …) on Vitest's expect, for the jsdom component project. Auto-cleanup between tests
// is handled by the svelteTesting() Vite plugin.
import '@testing-library/jest-dom/vitest';
