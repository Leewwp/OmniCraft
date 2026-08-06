import { buildOptions, defaultAction } from './lib/scenarios.js';

// Load: staged ramp to the target concurrency declared in the release
// profile, holding it for the profile duration, then a graceful ramp-down.
export const options = buildOptions(__ENV.PROFILE, 'Load');

export default function () {
  defaultAction(__ENV.PROFILE, 'Load');
}
