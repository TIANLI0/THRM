import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const curveSource = readFileSync(new URL('../src/app/components/FanCurve.tsx', import.meta.url), 'utf8');
const toolbarSource = readFileSync(new URL('../src/app/components/FanCurveProfileToolbar.tsx', import.meta.url), 'utf8');

test('uses the scrollable curve profile toolbar', () => {
  assert.match(curveSource, /data-curve-profile-row/);
  assert.match(toolbarSource, /data-curve-profile-toolbar/);
  assert.match(toolbarSource, /data-curve-profile-list/);
  assert.match(toolbarSource, /overflow-x-auto/);
  assert.match(toolbarSource, /items-center gap-1 overflow-x-auto/);
  assert.match(toolbarSource, /rounded-full border px-4/);
  assert.match(toolbarSource, /absolute -right-\[6\.5px\] -top-\[1\.5px\] z-10/);
  assert.match(toolbarSource, /border-primary bg-card text-destructive/);
  assert.match(toolbarSource, /border-border bg-card text-muted-foreground/);
  assert.doesNotMatch(toolbarSource, /bg-muted\/60|bg-destructive(?:\/15)?/);
  assert.match(toolbarSource, /group-hover:opacity-100/);
  assert.doesNotMatch(toolbarSource, /-ml-px flex h-\[13px\]/);
});

test('guards profile switching when the current curve is unsaved', () => {
  assert.match(curveSource, /if \(hasUnsavedChanges\) \{\s*setPendingProfileId\(id\);\s*setProfileSwitchDialogOpen\(true\);\s*return;/);
  assert.match(curveSource, /confirmProfileSwitch\('save'\)/);
  assert.match(curveSource, /confirmProfileSwitch\('discard'\)/);
});

test('keeps import and export string-only', () => {
  assert.match(curveSource, /exportFanCurveProfiles\(\)/);
  assert.match(curveSource, /ClipboardSetText\(code\)/);
  assert.match(curveSource, /importFanCurveProfiles\(code\)/);
  assert.match(curveSource, /value=\{importCode\}/);
  assert.doesNotMatch(curveSource, /type="file"|\.fcurve|FileReader|file\.text|onDrop|SaveFileDialog/);
});

test('renaming does not reload and discard an unsaved curve', () => {
  const renameHandler = curveSource.slice(
    curveSource.indexOf('const saveCurrentProfileName'),
    curveSource.indexOf('const createNewProfile'),
  );
  assert.match(renameHandler, /setCurveProfiles/);
  assert.doesNotMatch(renameHandler, /loadCurveProfiles/);
});

test('keeps the profile manager compact and single-layered', () => {
  const manager = curveSource.slice(
    curveSource.indexOf('<Dialog open={manageProfilesDialogOpen}'),
    curveSource.indexOf('<Dialog\n          open={profileSwitchDialogOpen}'),
  );
  assert.match(manager, /max-h-\[calc\(100vh-2rem\)\] max-w-lg gap-3 overflow-y-auto p-5/);
  assert.match(manager, /grid-cols-\[minmax\(0,1fr\)_auto\]/);
  assert.match(manager, /rows=\{2\}/);
  assert.doesNotMatch(manager, /data-profile-(?:export|import)-section className="[^"]*rounded-xl/);
});
