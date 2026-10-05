export interface WdttLink {
  name: string;
  host: string;      // peer address
  password: string;
  hashes: string[];
  workers?: number;
  port?: string;     // local listen port
}

export function parseWdttUrl(raw: string): WdttLink | null {
  try {
    const trimmed = raw.trim();

    // Новый формат: qwdtt://config?...
    if (trimmed.startsWith('qwdtt://config?')) {
      return parseQwdttConfig(trimmed);
    }

    // Старый формат: wdtt://...
    if (trimmed.startsWith('wdtt://')) {
      return parseLegacyWdtt(trimmed);
    }

    return null;
  } catch {
    return null;
  }
}

// Парсинг qwdtt://config?name=X&peer=IP:PORT&hashes=H1,H2&workers=N&port=PORT&pass=SECRET
function parseQwdttConfig(raw: string): WdttLink | null {
  const queryStr = raw.replace(/^qwdtt:\/\/config\?/, '');
  const params = new URLSearchParams(queryStr);

  const name = params.get('name') || 'Server';
  const peer = params.get('peer') || '';
  const pass = params.get('pass') || '';
  const hashesStr = params.get('hashes') || '';
  const workersStr = params.get('workers') || '';
  const port = params.get('port') || '';

  if (!peer || !pass) return null;

  const hashes = hashesStr.split(',').map(h => h.trim()).filter(Boolean);
  const workers = parseWorkers(workersStr);

  return { name, host: peer, password: pass, hashes, workers, port };
}

// workers валиден только как целое число в диапазоне 9–108 (4 хеша × 27).
// Любой мусор (NaN, вне диапазона, нечисловой) → undefined, без clamp.
function parseWorkers(workersStr: string): number | undefined {
  if (!workersStr) return undefined;
  const trimmed = workersStr.trim();
  if (!/^-?\d+$/.test(trimmed)) return undefined;
  const n = Number(trimmed);
  if (!Number.isFinite(n) || !Number.isInteger(n)) return undefined;
  if (n < 9 || n > 108) return undefined;
  return n;
}

// Парсинг старого формата: wdtt://IP:DTLS:WG:PROXY:PASSWORD[:HASHES][#name]
function parseLegacyWdtt(raw: string): WdttLink | null {
  const stripped = raw.replace(/^wdtt:\/\//, '');
  const parts = stripped.split(':');
  if (parts.length < 5) return null;

  const ip = parts[0];
  const dtlsPort = parts[1];
  const tail = parts.slice(4).join(':');

  let name = 'Server';
  const hashIdx = tail.lastIndexOf('#');
  let passwordAndHashes = tail;
  if (hashIdx !== -1) {
    const candidate = tail.slice(hashIdx + 1).trim();
    if (candidate) name = candidate;
    passwordAndHashes = tail.slice(0, hashIdx);
  }

  const colonIdx = passwordAndHashes.lastIndexOf(':');
  let password: string;
  let hashes: string[] = [];
  if (colonIdx !== -1) {
    password = passwordAndHashes.slice(0, colonIdx);
    hashes = passwordAndHashes.slice(colonIdx + 1).split(',').map(h => h.trim()).filter(Boolean);
  } else {
    password = passwordAndHashes;
  }

  if (!ip || !dtlsPort || !password) return null;
  return { name, host: `${ip}:${dtlsPort}`, password, hashes };
}

type Listener = (link: WdttLink | null) => void;
let pending: WdttLink | null = null;
const listeners = new Set<Listener>();

export const wdttLinkStore = {
  subscribe: (fn: Listener) => { listeners.add(fn); fn(pending); return () => { listeners.delete(fn); }; },
  set: (link: WdttLink | null) => { pending = link; listeners.forEach(fn => fn(link)); },
  consume: () => { const l = pending; pending = null; listeners.forEach(fn => fn(null)); return l; },
};
