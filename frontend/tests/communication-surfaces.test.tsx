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
  assert.match(chat, /<textarea/);
  assert.match(chat, /event\.key === "Enter" && !event\.shiftKey/);
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
