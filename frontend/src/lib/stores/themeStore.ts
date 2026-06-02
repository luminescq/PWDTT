const KEY = 'wdtt_theme';

type Theme = 'light' | 'dark';
type Listener = (t: Theme) => void;

let current: Theme = (localStorage.getItem(KEY) as Theme) ?? 'light';
const listeners = new Set<Listener>();

function apply(t: Theme) {
  document.documentElement.setAttribute('data-theme', t);
}

apply(current);

export const themeStore = {
  get: () => current,
  set: (t: Theme) => {
    current = t;
    localStorage.setItem(KEY, t);
    apply(t);
    listeners.forEach(fn => fn(t));
  },
  toggle: () => themeStore.set(current === 'light' ? 'dark' : 'light'),
  subscribe: (fn: Listener) => {
    listeners.add(fn);
    fn(current);
    return () => { listeners.delete(fn); };
  },
};
