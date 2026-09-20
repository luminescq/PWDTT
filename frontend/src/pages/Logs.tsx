import { useState, useEffect, useRef, useCallback } from 'react';
import { IconSearch, IconTrashX, IconCopy, IconCheck } from '@tabler/icons-react';
import { logStore, type LogEntry, type LogLevel } from '../lib/stores/logStore';
import './Logs.css';

type Filter = 'ALL' | 'INFO' | 'WARN' | 'ERROR' | 'DEBUG';

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
  const [copied, setCopied] = useState(false);
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

  const copiedTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handleCopy = () => {
    const text = visible.map(e => `[${e.time}] [${e.level}] ${e.message}${e.count > 1 ? ` (×${e.count})` : ''}`).join('\n');
    navigator.clipboard.writeText(text);
    setCopied(true);
    if (copiedTimerRef.current) clearTimeout(copiedTimerRef.current);
    copiedTimerRef.current = setTimeout(() => setCopied(false), 1500);
  };

  useEffect(() => {
    return () => { if (copiedTimerRef.current) clearTimeout(copiedTimerRef.current); };
  }, []);

  return (
    <>
      <main className="logs-main">
        <div className="logs-card">
          <div className="logs-toolbar">
            <div className="search-wrap">
              <div className="search-inner">
                <input
                  className="search-input"
                  placeholder="Поиск..."
                  value={search}
                  onChange={e => setSearch(e.target.value)}
                />
                <IconSearch size={18} className="search-icon" />
              </div>
            </div>
            <div className="logs-toolbar-right">
              <div className="filter-group">
                {(['ALL', 'INFO', 'WARN', 'ERROR', 'DEBUG'] as Filter[]).map(f => (
                  <button type="button" key={f} className={`filter-btn${filter === f ? ' filter-btn--active' : ''}`} onClick={() => setFilter(f)}>{f}</button>
                ))}
              </div>
              <button type="button" className="icon-btn" onClick={logStore.clear} title="Очистить" aria-label="Очистить логи">
                <IconTrashX stroke={2} size={16} />
              </button>
              <button type={`button`} className={`icon-btn${copied ? ' icon-btn--copied' : ''}`} onClick={handleCopy} title={copied ? 'Скопировано!' : 'Копировать'} aria-label="Копировать логи">
                {copied ? <IconCheck stroke={2} size={16} /> : <IconCopy stroke={2} size={16} />}
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