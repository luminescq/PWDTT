type Listener = (msg: string | null) => void;

let current: string | null = null;
let timer: ReturnType<typeof setTimeout> | null = null;
const listeners = new Set<Listener>();

function notify() { listeners.forEach(fn => fn(current)); }

export const toastStore = {
  subscribe: (fn: Listener) => {
    listeners.add(fn);
    fn(current);
    return () => { listeners.delete(fn); };
  },
  show: (msg: string, ms = 3000) => {
    if (timer) clearTimeout(timer);
    current = msg;
    notify();
    timer = setTimeout(() => { current = null; notify(); }, ms);
  },
};
