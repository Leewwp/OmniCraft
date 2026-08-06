import { buildOptions, defaultAction } from './lib/scenarios.js';

// Smoke: minimal traffic that validates the script, the critical paths and
// the thresholds wiring without loading the target.
export const options = buildOptions(__ENV.PROFILE, 'Smoke');

export default function () {
  defaultAction(__ENV.PROFILE, 'Smoke');
}
