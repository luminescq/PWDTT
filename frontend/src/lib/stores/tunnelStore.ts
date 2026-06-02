import type { TunnelState } from '../types';

type Listener = (state: TunnelState) => void;

let state: TunnelState = 'idle';
const listeners = new Set<Listener>();

export const tunnelStore = {
  get: () => state,
  set: (s: TunnelState) => {
    state = s;
    listeners.forEach(fn => fn(state));
  },
  subscribe: (fn: Listener) => {
    listeners.add(fn);
    fn(state);
    return () => { listeners.delete(fn); };
  },
};
