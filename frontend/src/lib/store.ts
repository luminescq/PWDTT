import type { Server, AppSettings, DeployConfig } from './types';
import { DEFAULT_SETTINGS, DEFAULT_DEPLOY } from './types';
import { Encrypt, Decrypt } from '../../wailsjs/go/backend/App';

const SERVERS_KEY = 'wdtt_servers';
const SETTINGS_KEY = 'wdtt_settings';
const LAST_SERVER_KEY = 'wdtt_last_server';
const ENC_PREFIX = 'enc:';
const DEPLOY_KEY = 'wdtt_deploy';

function parse<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : fallback;
  } catch {
    return fallback;
  }
}

async function encryptPassword(pw: string): Promise<string> {
  if (!pw || pw.startsWith(ENC_PREFIX)) return pw;
  try {
    return ENC_PREFIX + (await Encrypt(pw));
  } catch {
    return pw;
  }
}

async function decryptPassword(pw: string): Promise<string> {
  if (!pw || !pw.startsWith(ENC_PREFIX)) return pw;
  try {
    return await Decrypt(pw.slice(ENC_PREFIX.length));
  } catch {
    return pw;
  }
}

export const serverStore = {
  getAll: async (): Promise<Server[]> => {
    const servers = parse<Server[]>(SERVERS_KEY, []);
    return Promise.all(servers.map(async s => ({
      ...s,
      password: await decryptPassword(s.password),
    })));
  },
  save: async (servers: Server[]) => {
    const encrypted = await Promise.all(servers.map(async s => ({
      ...s,
      password: await encryptPassword(s.password),
    })));
    localStorage.setItem(SERVERS_KEY, JSON.stringify(encrypted));
  },
  add: async (server: Omit<Server, 'id'>): Promise<Server> => {
    const s: Server = { ...server, id: crypto.randomUUID() };
    const all = await serverStore.getAll();
    await serverStore.save([...all, s]);
    return s;
  },
  update: async (server: Server) => {
    const all = await serverStore.getAll();
    await serverStore.save(all.map(s => s.id === server.id ? server : s));
  },
  remove: async (id: string) => {
    const all = await serverStore.getAll();
    await serverStore.save(all.filter(s => s.id !== id));
  },
  getLastSelectedId: (): string | null => parse<string | null>(LAST_SERVER_KEY, null),
  setLastSelectedId: (id: string | null) => {
    if (id) localStorage.setItem(LAST_SERVER_KEY, JSON.stringify(id));
    else localStorage.removeItem(LAST_SERVER_KEY);
  },
};

export const settingsStore = {
  get: (): AppSettings => {
    const saved = parse<Partial<AppSettings>>(SETTINGS_KEY, {});
    const merged = { ...DEFAULT_SETTINGS, ...saved };
    const h = Array.isArray(merged.hashes) ? merged.hashes : [];
    merged.hashes = [h[0] ?? '', h[1] ?? '', h[2] ?? '', h[3] ?? ''];
    return merged;
  },
  save: (settings: AppSettings) => localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings)),
};

export const deployStore = {
  get: async (): Promise<DeployConfig> => {
    const cfg = parse<DeployConfig>(DEPLOY_KEY, DEFAULT_DEPLOY);
    return {
      ...cfg,
      password: await decryptPassword(cfg.password),
      tunnelPassword: await decryptPassword(cfg.tunnelPassword),
      tgBotToken: cfg.tgBotToken ? await decryptPassword(cfg.tgBotToken) : '',
    };
  },
  save: async (cfg: DeployConfig) => {
    const encrypted: DeployConfig = {
      ...cfg,
      password: await encryptPassword(cfg.password),
      tunnelPassword: await encryptPassword(cfg.tunnelPassword),
      tgBotToken: cfg.tgBotToken ? await encryptPassword(cfg.tgBotToken) : '',
    };
    localStorage.setItem(DEPLOY_KEY, JSON.stringify(encrypted));
  },
};
