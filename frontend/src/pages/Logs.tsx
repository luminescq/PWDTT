import { useState, useEffect, useRef, useCallback } from 'react';
import { IconSearch, IconTrashX, IconCopy } from '@tabler/icons-react';
import { logStore, type LogEntry, type LogLevel } from '../lib/stores/logStore';

type Filter = 'ALL' | 'INFO' | 'ERROR';

const LEVEL_COLOR: Record<LogLevel, string> = {
  INFO:  'var(--text)',
  WARN:  '#f59e0b',
  ERROR: '#ef4444',
  DEBUG: 'var(--text-3)',
};

export default function Logs() {
  const [filter, setFilter] = useState<Filter>('ALL');
  const [search, setSearch] = useState('');
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const bottomRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const autoScroll = useRef(true);

  useEffect(() => logStore.subscribe(setEntries), []);

  useEffect(() => {
    if (autoScroll.current) bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [entries]);

  const onScroll = useCallback(() => {
    const el = listRef.current;
    if (!el) return;
    autoScroll.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  }, []);

  const visible = entries.filter(e => {
    if (filter !== 'ALL' && e.level !== filter) return false;
    if (search && !e.message.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

  const handleCopy = () => {
    const text = visible.map(e => `[${e.time}] [${e.level}] ${e.message}${e.count > 1 ? ` (×${e.count})` : ''}`).join('\n');
    navigator.clipboard.writeText(text);
  };

  return (
    <>
      <style>{`
        .logs-main { flex: 1; min-height: 0; padding: 2px 2px 2px 0; display: flex; flex-direction: column; animation: page-in 0.25s ease-out; }
        .logs-card { flex: 1; min-height: 0; border: 1px solid var(--border); border-radius: 12px; display: flex; flex-direction: column; overflow: hidden; background: var(--surface); }
        .logs-toolbar { display: flex; align-items: center; gap: 10px; padding: 14px 16px; border-bottom: 1px solid var(--border-2); flex-shrink: 0; }
        .search-wrap { flex: 1; display: flex; justify-content: center; min-width: 0; }
        .search-inner { position: relative; width: 100%; max-width: 380px; }
        .search-input { width: 100%; padding: 8px 36px 8px 14px; border: 1px solid var(--input-border); border-radius: 12px; background: var(--input-bg); font-size: 14px; color: var(--text); outline: none; box-sizing: border-box; }
        .search-input::placeholder { color: var(--text-4); }
        .search-icon { position: absolute; right: 10px; top: 50%; transform: translateY(-50%); color: var(--text-3); pointer-events: none; }
        .logs-toolbar-right { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
        .filter-group { display: flex; background: var(--seg-bg); border-radius: 10px; padding: 3px; gap: 2px; }
        .filter-btn { padding: 6px 16px; border: none; border-radius: 8px; background: transparent; font-size: 13px; font-weight: 600; color: var(--seg-text); cursor: pointer; transition: background 0.15s, color 0.15s; }
        .filter-btn--active { background: var(--accent); color: var(--accent-fg); }
        .icon-btn { width: 36px; height: 36px; border: 0.5px solid var(--border); border-radius: 10px; background: transparent; cursor: pointer; display: flex; align-items: center; justify-content: center; color: var(--text-3); transition: background 0.12s, border-color 0.12s, color 0.12s; }
        .icon-btn:hover { background: var(--bg-3); border-color: var(--text-3); color: var(--text); }
        .logs-list { flex: 1; min-height: 0; overflow-y: auto; padding: 8px 0; }
        .log-row { display: flex; align-items: baseline; gap: 10px; padding: 4px 16px; font-size: 13px; line-height: 1.5; }
        .log-row:hover { background: var(--bg-2); }
        .log-time { color: var(--text-4); flex-shrink: 0; font-size: 12px; font-variant-numeric: tabular-nums; }
        .log-level { flex-shrink: 0; font-weight: 700; font-size: 12px; width: 44px; }
        .log-msg { flex: 1; word-break: break-all; color: var(--text); }
        .log-count { flex-shrink: 0; background: var(--seg-bg); border-radius: 20px; padding: 1px 7px; font-size: 11px; color: var(--text-2); }
        .logs-empty { flex: 1; display: flex; align-items: center; justify-content: center; color: var(--text-4); font-size: 15px; }
      `}</style>
      <main className="logs-main">
        <div className="logs-card">
          <div className="logs-toolbar">
            <div className="search-wrap">
              <div className="search-inner">
                <input
                  className="search-input"
                  placeholder="Поиск...."
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                />
                <IconSearch size={18} className="search-icon" />
              </div>
            </div>
            <div className="logs-toolbar-right">
              <div className="filter-group">
                {(['ALL', 'INFO', 'ERROR'] as Filter[]).map(f => (
                  <button key={f} className={`filter-btn${filter === f ? ' filter-btn--active' : ''}`} onClick={() => setFilter(f)}>{f}</button>
                ))}
              </div>
              <button className="icon-btn" onClick={logStore.clear} title="Очистить" aria-label="Очистить логи">
                <IconTrashX stroke={2} size={16} />
              </button>
              <button className="icon-btn" onClick={handleCopy} title="Копировать" aria-label="Копировать логи">
                <IconCopy stroke={2} size={16} />
              </button>
            </div>
          </div>

          {visible.length === 0 ? (
            <div className="logs-empty">{entries.length === 0 ? 'Логи появятся здесь...' : 'Ничего не найдено'}</div>
          ) : (
            <div className="logs-list" ref={listRef} onScroll={onScroll}>
              {visible.map(e => (
                <div key={e.id} className="log-row">
                  <span className="log-time">{e.time}</span>
                  <span className="log-level" style={{ color: LEVEL_COLOR[e.level] }}>{e.level}</span>
                  <span className="log-msg">{e.message}</span>
                  {e.count > 1 && <span className="log-count">×{e.count}</span>}
                </div>
              ))}
              <div ref={bottomRef} />
            </div>
          )}
        </div>
      </main>
    </>
  );
}