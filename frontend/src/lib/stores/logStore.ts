export type LogLevel = 'INFO' | 'ERROR' | 'WARN' | 'DEBUG';

export interface LogEntry {
  id: number;
  level: LogLevel;
  message: string;
  time: string;
  count: number;
}

type Listener = (entries: LogEntry[]) => void;

let seq = 0;
let entries: LogEntry[] = [];
const listeners = new Set<Listener>();
const MAX_ENTRIES = 500;

function notify() {
  listeners.forEach(fn => fn([...entries]));
}

function extractTag(message: string): string {
  const m = message.match(/^\[([^\]]+)\]/);
  if (!m) return '';
  return m[1].replace(/\s*#\d+$/, '').replace(/\s+\d+$/, '');
}

export const logStore = {
  subscribe: (fn: Listener) => {
    listeners.add(fn);
    fn([...entries]);
    return () => { listeners.delete(fn); };
  },

  push: (level: LogLevel, message: string) => {
    const time = new Date().toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

    if (level === 'ERROR') {
      entries = [...entries, { id: seq++, level, message, time, count: 1 }];
      if (entries.length > MAX_ENTRIES) entries = entries.slice(-MAX_ENTRIES);
      notify();
      return;
    }

    const tag = extractTag(message);
    if (tag) {
      const idx = entries.findIndex(e => e.level === level && extractTag(e.message) === tag);
      if (idx !== -1) {
        const found = entries[idx];
        entries = [...entries.slice(0, idx), { ...found, message, time, count: found.count + 1 }, ...entries.slice(idx + 1)];
        notify();
        return;
      }
    }

    const last = entries[entries.length - 1];
    if (last && last.message === message && last.level === level) {
      entries = [...entries.slice(0, -1), { ...last, count: last.count + 1 }];
      notify();
      return;
    }

    entries = [...entries, { id: seq++, level, message, time, count: 1 }];
    if (entries.length > MAX_ENTRIES) entries = entries.slice(-MAX_ENTRIES);
    notify();
  },

  clear: () => {
    entries = [];
    notify();
  },

  getAll: () => [...entries],
};
