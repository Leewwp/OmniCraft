import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";

const frontendRoot = path.resolve(import.meta.dirname, "..");

async function source(relativePath: string) {
  return readFile(path.join(frontendRoot, relativePath), "utf8");
}

test("communication layouts localize visible guard and skip-link copy", async () => {
  const [rootLayout, protectedLayout] = await Promise.all([
    source("app/layout.tsx"),
    source("app/(protected)/layout.tsx"),
  ]);
  assert.doesNotMatch(rootLayout, />\s*Skip to content\s*</);
  assert.match(rootLayout, /t\('skipToContent'\)/);
  assert.doesNotMatch(protectedLayout, /Account Suspended|Submit an appeal/);
  assert.match(protectedLayout, /t\("suspendedTitle"\)/);
});

test("message center uses semantic tabs and the approved responsive columns", async () => {
  const page = await source("app/(protected)/messages/page.tsx");
  assert.match(page, /role="tablist"/);
  assert.match(page, /aria-selected=\{tab === tKey\}/);
  assert.match(page, /min-\[701px\]:grid-cols-\[280px_minmax\(0,1fr\)\]/);
  assert.match(page, /min-\[1101px\]:grid-cols-\[320px_minmax\(0,1fr\)\]/);
  assert.doesNotMatch(page, /style=\{\{/);
});

test("conversation and chat surfaces preserve endpoints and expose stable states", async () => {
  const [conversations, chat] = await Promise.all([
    source("components/social/ConversationList.tsx"),
    source("components/social/ChatWindow.tsx"),
  ]);
  assert.match(conversations, /"\/api\/v1\/messages"/);
  assert.match(conversations, /min-h-16/);
  assert.match(conversations, /unreadCount > 99 \? "99\+"/);
  assert.match(chat, /`\/api\/v1\/messages\/\$\{conversationId\}`/);
  assert.match(chat, /recipient_id: recipient\.id, text: body/);
  assert.match(chat, /role="log"/);
  // #413 F6a：输入面收敛为公共 Composer（Enter 发送 / Shift+Enter 换行由
  // keyMode="enter" 显式声明，行为契约由 tests/composer.test.tsx 覆盖）。
  assert.match(chat, /<Composer/);
  assert.match(chat, /keyMode="enter"/);
  assert.match(chat, /messages\.chat\.unsupportedMessage/);
  assert.doesNotMatch(chat, /err\.message|error\.message/);
});

test("notification dropdown uses semantic Indigo contrast and labeled status", async () => {
  const dropdown = await source("components/social/NotificationDropdown.tsx");
  assert.match(dropdown, /bg-accent-subtle/);
  assert.match(dropdown, /text-accent-emphasis/);
  assert.match(dropdown, /messages\.broadcast\.label/);
  assert.match(dropdown, /aria-expanded=\{open\}/);
  assert.doesNotMatch(dropdown, /shadow-md|bg-accent\/5|text-accent\s/);
});
