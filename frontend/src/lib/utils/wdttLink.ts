export interface WdttLink {
  ip: string;
  dtlsPort: string;
  password: string;
  hashes: string[];
  name: string; // из #название или 'Server'
}

// wdtt://<IP>:<DTLS>:<WG>:<PROXY>:<PASSWORD>[:<HASH1>,<HASH2>,...][#название]
export function parseWdttUrl(raw: string): WdttLink | null {
  try {
    let str = raw.trim();
    // извлекаем #название
    let name = 'Server';
    const hashIdx = str.indexOf('#');
    if (hashIdx !== -1) {
      const candidate = str.slice(hashIdx + 1).trim();
      if (candidate) name = candidate;
      str = str.slice(0, hashIdx);
    }
    const stripped = str.replace(/^wdtt:\/\//, '');
    const parts = stripped.split(':');
    if (parts.length < 5) return null;
    const ip = parts[0];
    const dtlsPort = parts[1];
    const password = parts[4];
    const hashes = parts[5]
      ? parts[5].split(',').map(h => h.trim()).filter(Boolean)
      : [];
    if (!ip || !dtlsPort || !password) return null;
    return { ip, dtlsPort, password, hashes, name };
  } catch {
    return null;
  }
}

type Listener = (link: WdttLink | null) => void;
let pending: WdttLink | null = null;
const listeners = new Set<Listener>();

export const wdttLinkStore = {
  subscribe: (fn: Listener) => { listeners.add(fn); fn(pending); return () => { listeners.delete(fn); }; },
  set: (link: WdttLink | null) => { pending = link; listeners.forEach(fn => fn(link)); },
  consume: () => { const l = pending; pending = null; listeners.forEach(fn => fn(null)); return l; },
};
