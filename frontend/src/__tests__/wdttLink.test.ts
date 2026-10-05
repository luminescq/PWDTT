import { describe, it, expect } from 'vitest';
import { parseWdttUrl } from '../lib/utils/wdttLink';

describe('parseWdttUrl', () => {
  // ── qwdtt://config? формат ──

  it('полный формат qwdtt с хешами и названием', () => {
    const url = 'qwdtt://config?name=MyServer&peer=1.2.3.4:5555&pass=secret&hashes=h1,h2,h3&workers=18&port=9000';
    const link = parseWdttUrl(url);

    expect(link).not.toBeNull();
    expect(link!.name).toBe('MyServer');
    expect(link!.host).toBe('1.2.3.4:5555');
    expect(link!.password).toBe('secret');
    expect(link!.hashes).toEqual(['h1', 'h2', 'h3']);
    expect(link!.workers).toBe(18);
    expect(link!.port).toBe('9000');
  });

  it('без хешей', () => {
    const url = 'qwdtt://config?peer=1.2.3.4:5555&pass=secret';
    const link = parseWdttUrl(url);

    expect(link).not.toBeNull();
    expect(link!.hashes).toEqual([]);
  });

  it('без названия — дефолт "Server"', () => {
    const url = 'qwdtt://config?peer=1.2.3.4:5555&pass=secret';
    const link = parseWdttUrl(url);

    expect(link!.name).toBe('Server');
  });

  it('без peer → null', () => {
    const url = 'qwdtt://config?pass=secret';
    expect(parseWdttUrl(url)).toBeNull();
  });

  it('без pass → null', () => {
    const url = 'qwdtt://config?peer=1.2.3.4:5555';
    expect(parseWdttUrl(url)).toBeNull();
  });

  it('множественные хеши через запятую с пробелами', () => {
    const url = 'qwdtt://config?peer=1.2.3.4:5555&pass=x&hashes= h1 , h2 , h3 ';
    const link = parseWdttUrl(url);

    expect(link!.hashes).toEqual(['h1', 'h2', 'h3']);
  });

  it('обрезка пробелов в URL', () => {
    const url = '  qwdtt://config?peer=1.2.3.4:5555&pass=secret  ';
    const link = parseWdttUrl(url);

    expect(link).not.toBeNull();
    expect(link!.host).toBe('1.2.3.4:5555');
  });

  // ── wdtt:// legacy формат ──

  it('legacy формат с хешами и названием', () => {
    const url = 'wdtt://1.2.3.4:5555:9:10:password:hash1,hash2#MyServer';
    const link = parseWdttUrl(url);

    expect(link).not.toBeNull();
    expect(link!.host).toBe('1.2.3.4:5555');
    expect(link!.password).toBe('password');
    expect(link!.hashes).toEqual(['hash1', 'hash2']);
    expect(link!.name).toBe('MyServer');
  });

  it('legacy формат без названия — дефолт "Server"', () => {
    const url = 'wdtt://1.2.3.4:5555:9:10:password:hash1';
    const link = parseWdttUrl(url);

    expect(link!.name).toBe('Server');
  });

  it('legacy формат без хешей', () => {
    const url = 'wdtt://1.2.3.4:5555:9:10:password';
    const link = parseWdttUrl(url);

    expect(link).not.toBeNull();
    expect(link!.password).toBe('password');
    expect(link!.hashes).toEqual([]);
  });

  // ── некорректные форматы ──

  it('пустая строка → null', () => {
    expect(parseWdttUrl('')).toBeNull();
  });

  it('короткий формат → null', () => {
    expect(parseWdttUrl('wdtt://short')).toBeNull();
  });

  it('неизвестный протокол → null', () => {
    expect(parseWdttUrl('http://example.com')).toBeNull();
  });

  it('legacy слишком мало частей → null', () => {
    expect(parseWdttUrl('wdtt://1.2.3.4:5555')).toBeNull();
  });

  it('qwdtt без query string → null', () => {
    expect(parseWdttUrl('qwdtt://config')).toBeNull();
  });

  it('workers=abc → undefined', () => {
    const link = parseWdttUrl('qwdtt://config?peer=1.2.3.4:5555&pass=secret&workers=abc');
    expect(link).not.toBeNull();
    expect(link!.workers).toBeUndefined();
  });

  it('workers=-5 → undefined', () => {
    const link = parseWdttUrl('qwdtt://config?peer=1.2.3.4:5555&pass=secret&workers=-5');
    expect(link).not.toBeNull();
    expect(link!.workers).toBeUndefined();
  });

  it('workers=9999 → undefined', () => {
    const link = parseWdttUrl('qwdtt://config?peer=1.2.3.4:5555&pass=secret&workers=9999');
    expect(link).not.toBeNull();
    expect(link!.workers).toBeUndefined();
  });
});
