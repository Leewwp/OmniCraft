import { buildOptions, defaultAction } from './lib/scenarios.js';

// Stress: aggressive step ramp well beyond the load target to find the
// bottleneck and observe recovery behaviour.
export const options = buildOptions(__ENV.PROFILE, 'Stress');

export default function () {
  defaultAction(__ENV.PROFILE, 'Stress');
}
