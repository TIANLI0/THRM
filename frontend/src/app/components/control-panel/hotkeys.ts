export function normalizeHotkeyForDisplay(value: string): string {
  return (value || '').trim();
}

export function buildShortcutFromKeyboardEvent(e: {
  key: string;
  ctrlKey: boolean;
  altKey: boolean;
  shiftKey: boolean;
  metaKey: boolean;
}): string {
  if (e.key === 'Backspace' || e.key === 'Delete') {
    return '';
  }

  const parts: string[] = [];
  if (e.ctrlKey) parts.push('Ctrl');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  if (e.metaKey) parts.push('Win');

  const key = e.key;
  if (['Control', 'Alt', 'Shift', 'Meta'].includes(key)) {
    return '';
  }

  let mainKey = '';
  if (/^[a-z]$/i.test(key)) {
    mainKey = key.toUpperCase();
  } else if (/^[0-9]$/.test(key)) {
    mainKey = key;
  } else if (/^F\d{1,2}$/i.test(key)) {
    mainKey = key.toUpperCase();
  }

  if (!mainKey || parts.length === 0) {
    return '';
  }

  return [...parts, mainKey].join('+');
}
