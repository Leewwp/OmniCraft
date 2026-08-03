import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";

import { AuthProvider } from "@/contexts/AuthContext";
import { ApiRequestError, api, setAccessToken } from "@/lib/api";
import { ChatWindow } from "@/components/social/ChatWindow";
import { ConversationList } from "@/components/social/ConversationList";
import { NotificationList } from "@/components/social/NotificationList";
import { ToastProvider } from "@/components/ui/Toast";

import { act, cleanup, fireEvent, installDom, renderWithIntl, waitFor } from "./runtime-test-helpers";

const originalGet = api.get;
const originalPost = api.post;
const originalPatch = api.patch;

const user = {
  id: 1,
  email: "alice@example.test",
  username: "alice",
  avatar_url: "",
  bio: "",
  reputation: 10,
  preferred_locale: "en",
  role: "user",
  is_banned: false,
  email_verified_at: "2026-06-30T00:00:00Z",
  created_at: "2026-06-30T00:00:00Z",
};

const conversation = {
  id: 42,
  participants: [
    { id: 1, username: "alice", avatar_url: "" },
    { id: 2, username: "bob", avatar_url: "" },
  ],
  last_message: {
    id: 10,
    text: "hello",
    sender_id: 2,
    created_at: "2026-06-30T12:01:00Z",
  },
  unread_count: 1,
  updated_at: "2026-06-30T12:02:00Z",
};

const intlMessages = {
  common: {
    close: "Close",
    back: "Back",
    loading: "Loading...",
  },
  discussion: {
    search: "Search conversations",
  },
  notification: {
    all: "All",
    reply: "Replies",
    like: "Likes",
    channelReply: "Replies",
    channelLike: "Likes",
    system: "System",
    channelFollow: "Follows",
    channelPR: "PR",
    channelBroadcast: "Broadcast",
  },
  messages: {
    noConversations: "No conversations",
    noMessages: "No messages",
    markAllRead: "Mark all read",
    read: "Read",
    selectConversation: "Select a conversation",
    replyPlaceholder: "Type a message...",
    startConversation: "Start a conversation",
    dmReplyRequired: "Wait for the recipient to reply before sending another message.",
    sendMessage: "Send message",
    broadcastLabel: "Broadcast",
    systemLabel: "System",
    notificationOpensTarget: "Opens related item",
    notificationNoTarget: "No related item",
    unreadIndicator: "Unread",
    a11y: {
      conversationList: "Conversations",
      messageList: "Conversation messages",
    },
    conversations: {
      searchLabel: "Search conversations",
      retry: "Retry conversations",
      emptyTitle: "No conversations",
      emptyDescription: "Start a conversation from a creator or content page.",
      collabInviteSummary: "Collaboration invitation",
      unknownParticipant: "Unknown participant",
      unreadCount: "{count} unread",
      timeUnknown: "Time unavailable",
      startConversation: "Start a conversation",
    },
    chat: {
      backToConversations: "Back to conversations",
      recipientUnavailable: "Recipient unavailable",
      retry: "Retry messages",
      emptyTitle: "No messages yet",
      emptyDescription: "Send a message to start this conversation.",
      unsupportedMessage: "This message type is not supported yet.",
      inputLabel: "Message",
      send: "Send message",
      sending: "Sending",
      timeUnknown: "Time unavailable",
      inputPlaceholder: "Type a message...",
      selectConversation: "Select a conversation",
      collabInviteSummary: "Collaboration invitation",
      replyRequired: "Wait for the recipient to reply before sending another message.",
    },
    error: {
      conversations: "Could not load conversations.",
      chat: "Could not load this conversation.",
      send: "Could not send the message.",
    },
  },
};

type ApiCall = {
  path: string;
  body?: unknown;
};

type MockOptions = {
  detailMessages?: Array<{ id: number; sender_id: number; text: string; body: string; created_at: string }>;
  notifications?: Array<{
    id: number;
    type: string;
    channel: string;
    title?: string;
    body: string;
    is_read: boolean;
    target_type?: string;
    target_id?: number;
    created_at: string;
  }>;
  postPromise?: Promise<unknown>;
  postError?: Error;
};

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.post = originalPost;
  api.patch = originalPatch;
  setAccessToken(null);
});

test("ConversationList calls the message center conversation endpoint", async () => {
  installMessagesDom();
  const calls = installApiMocks();

  renderMessagesComponent(<ConversationList onSelect={() => {}} />);

  await waitFor(() => {
    assert.ok(calls.get.some((call) => call.path === "/api/v1/messages"));
  });
});

test("ChatWindow loads messages from the message center detail endpoint", async () => {
  installMessagesDom();
  const calls = installApiMocks();

  renderMessagesComponent(<ChatWindow conversation={conversation} />);

  await waitFor(() => {
    assert.ok(calls.get.some((call) => call.path === "/api/v1/messages/42"));
  });
});

test("ChatWindow sends replies through the message center send endpoint", async () => {
  installMessagesDom();
  const calls = installApiMocks();

  const view = renderMessagesComponent(<ChatWindow conversation={conversation} />);

  const input = await findReplyInput(view.container);
  fireEvent.change(input, { target: { value: "hello back" } });
  await waitFor(() => assert.equal(input.value, "hello back"));

  const sendButton = view.getByLabelText(intlMessages.messages.chat.send);
  fireEvent.click(sendButton);

  await waitFor(() => {
    const sendCall = calls.post.find((call) => call.path === "/api/v1/messages");
    assert.ok(sendCall, `expected POST /api/v1/messages, got ${JSON.stringify(calls.post)}`);
    assert.deepEqual(sendCall.body, { recipient_id: 2, text: "hello back" });
  });
});

test("ChatWindow blocks duplicate sends while the first send is pending", async () => {
  installMessagesDom();
  let resolvePost: (value: unknown) => void = () => {};
  const pendingPost = new Promise((resolve) => {
    resolvePost = resolve;
  });
  const calls = installApiMocks({ postPromise: pendingPost });

  const view = renderMessagesComponent(<ChatWindow conversation={conversation} />);

  const input = await findReplyInput(view.container);
  fireEvent.change(input, { target: { value: "hello back" } });
  await waitFor(() => assert.equal(input.value, "hello back"));

  const sendButton = view.getByLabelText(intlMessages.messages.chat.send) as HTMLButtonElement;
  fireEvent.click(sendButton);
  fireEvent.click(sendButton);

  await waitFor(() => {
    const sendCalls = calls.post.filter((call) => call.path === "/api/v1/messages");
    assert.equal(sendCalls.length, 1, `expected one pending send, got ${JSON.stringify(sendCalls)}`);
    assert.equal(sendButton.disabled, true, "send button should be disabled while pending");
  });

  await act(async () => {
    resolvePost({
      message: { id: 11, sender_id: 1, text: "hello back", body: "hello back", created_at: "2026-06-30T12:03:00Z" },
    });
    await pendingPost;
  });
});

test("ChatWindow keeps the draft when the DM reply guard rejects the send", async () => {
  installMessagesDom();
  installApiMocks({
    postError: new ApiRequestError("DM_REPLY_REQUIRED", "reply required", 403),
  });

  const view = renderMessagesComponent(<ChatWindow conversation={conversation} />);

  const input = await findReplyInput(view.container);
  fireEvent.change(input, { target: { value: "second ping" } });
  await waitFor(() => assert.equal(input.value, "second ping"));

  const sendButton = view.getByLabelText(intlMessages.messages.chat.send);
  fireEvent.click(sendButton);

  await waitFor(() => {
    assert.ok(
      document.body.textContent?.includes(intlMessages.messages.dmReplyRequired),
      "localized DM reply toast should render",
    );
    assert.equal(input.value, "second ping", "draft should remain after rejected send");
  });
});

test("ChatWindow renders loaded messages in chronological order", async () => {
  installMessagesDom();
  installApiMocks({
    detailMessages: [
      { id: 12, sender_id: 2, text: "newest", body: "newest", created_at: "2026-06-30T12:03:00Z" },
      { id: 10, sender_id: 2, text: "oldest", body: "oldest", created_at: "2026-06-30T12:01:00Z" },
    ],
  });

  const view = renderMessagesComponent(<ChatWindow conversation={conversation} />);

  await waitFor(() => {
    const text = view.container.textContent ?? "";
    const oldestIndex = text.indexOf("oldest");
    const newestIndex = text.indexOf("newest");
    assert.ok(oldestIndex >= 0, "oldest message should render");
    assert.ok(newestIndex >= 0, "newest message should render");
    assert.ok(oldestIndex < newestIndex, "oldest message should render before newest message");
  });
});

test("ChatWindow shows a localized toast when the backend requires a DM reply", async () => {
  installMessagesDom();
  installApiMocks({
    postError: new ApiRequestError("DM_REPLY_REQUIRED", "reply required", 403),
  });

  const view = renderMessagesComponent(<ChatWindow conversation={conversation} />);

  const input = await findReplyInput(view.container);
  fireEvent.change(input, { target: { value: "second ping" } });
  await waitFor(() => assert.equal(input.value, "second ping"));

  const sendButton = view.getByLabelText(intlMessages.messages.chat.send);
  fireEvent.click(sendButton);

  await waitFor(() => {
    assert.ok(
      document.body.textContent?.includes(intlMessages.messages.dmReplyRequired),
      "localized DM reply toast should render",
    );
  });
});

test("ChatWindow mobile back control uses only localized text", async () => {
  installMessagesDom();
  installApiMocks();

  const view = renderMessagesComponent(<ChatWindow conversation={conversation} onBack={() => undefined} />);

  await waitFor(() => {
    assert.ok(view.getByRole("button", { name: intlMessages.common.back }));
  });
  assert.equal(view.container.textContent?.includes("鈫"), false);
});

test("NotificationList renders broadcast notifications with accent marker, title, and safe Markdown", async () => {
  installMessagesDom();
  installApiMocks({
    notifications: [
      {
        id: 77,
        type: "system",
        channel: "broadcast",
        title: "Maintenance window",
        body: "**Downtime** from 02:00. <script>alert('x')</script>",
        is_read: false,
        created_at: "2026-06-30T12:05:00Z",
      },
    ],
  });

  const view = renderMessagesComponent(<NotificationList />);

  const item = await waitFor(() => {
    const element = view.getByLabelText(/Broadcast.*No related item/);
    assert.ok(element.className.includes("border-l-blue") || element.className.includes("border-l-accent"));
    return element;
  });

  assert.ok(view.getByText("Maintenance window"));
  assert.ok(view.container.querySelector("strong")?.textContent?.includes("Downtime"));
  assert.equal(view.container.querySelector("script"), null);
  assert.ok(item.className.includes("cursor-default"), "broadcast without a target should not look like a link");
});

test("NotificationList keeps only valid notification targets navigation-clickable", async () => {
  installMessagesDom();
  installApiMocks({
    notifications: [
      {
        id: 78,
        type: "system",
        channel: "broadcast",
        title: "Release notes",
        body: "Read the update",
        is_read: true,
        target_type: "content",
        target_id: 100,
        created_at: "2026-06-30T12:06:00Z",
      },
    ],
  });

  const view = renderMessagesComponent(<NotificationList />);

  const item = await waitFor(() => view.getByLabelText(/Broadcast.*Opens related item/));
  assert.ok(item.className.includes("cursor-pointer"), "broadcast with a valid target should stay clickable");
});

test("NotificationList treats invalid target ids as non-clickable", async () => {
  installMessagesDom();
  installApiMocks({
    notifications: [
      {
        id: 79,
        type: "system",
        channel: "broadcast",
        title: "Broken target",
        body: "This should not navigate",
        is_read: true,
        target_type: "content",
        target_id: -1,
        created_at: "2026-06-30T12:07:00Z",
      },
    ],
  });

  const view = renderMessagesComponent(<NotificationList />);

  const item = await waitFor(() => view.getByLabelText(/Broadcast.*No related item/));
  assert.ok(item.className.includes("cursor-default"), "negative target ids should not create navigation");
});

test("NotificationList updates unread count callback after marking a notification read", async () => {
  installMessagesDom();
  installApiMocks({
    notifications: [
      {
        id: 80,
        type: "system",
        channel: "broadcast",
        title: "Unread broadcast",
        body: "Please read this",
        is_read: false,
        created_at: "2026-06-30T12:08:00Z",
      },
    ],
  });
  const unreadCounts: number[] = [];

  const view = renderMessagesComponent(
    <NotificationList onUnreadCountChange={(count) => unreadCounts.push(count)} />,
  );

  await waitFor(() => {
    assert.equal(unreadCounts.at(-1), 1);
  });
  fireEvent.click(view.getByRole("button", { name: intlMessages.messages.read }));

  await waitFor(() => {
    assert.equal(unreadCounts.at(-1), 0);
  });
});

test("NotificationList ignores stale responses from earlier channel loads", async () => {
  installMessagesDom();
  let resolveAllNotifications: (value: unknown) => void = () => undefined;
  const allNotifications = new Promise((resolve) => {
    resolveAllNotifications = resolve;
  });

  api.get = (async <T,>(path: string): Promise<T> => {
    if (path === "/api/v1/auth/me") {
      return { user } as T;
    }
    if (path === "/api/v1/notifications/unread-count") {
      return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } } as T;
    }
    if (path === "/api/v1/notifications") {
      return (await allNotifications) as T;
    }
    if (path === "/api/v1/notifications?channel=broadcast") {
      return {
        notifications: [
          {
            id: 81,
            type: "system",
            channel: "broadcast",
            title: "Fresh broadcast",
            body: "Fresh body",
            is_read: true,
            created_at: "2026-06-30T12:09:00Z",
          },
        ],
      } as T;
    }
    throw new Error(`unexpected api.get path ${path}`);
  }) as typeof api.get;
  api.patch = (async <T,>(): Promise<T> => ({} as T)) as typeof api.patch;

  const view = renderMessagesComponent(<NotificationList />);

  fireEvent.click(view.getByRole("button", { name: intlMessages.notification.channelBroadcast }));
  await waitFor(() => {
    assert.ok(view.getByText("Fresh broadcast"));
  });

  await act(async () => {
    resolveAllNotifications({
      notifications: [
        {
          id: 82,
          type: "reply",
          channel: "reply",
          title: "Stale reply",
          body: "Old body",
          is_read: true,
          created_at: "2026-06-30T12:01:00Z",
        },
      ],
    });
    await allNotifications;
  });

  await waitFor(() => {
    assert.equal(view.queryByText("Stale reply"), null);
    assert.ok(view.getByText("Fresh broadcast"));
  });
});

function renderMessagesComponent(node: React.ReactNode) {
  setValidAccessToken();
  return renderWithIntl(
    <IntlProvider locale="en" messages={intlMessages}>
      <AppRouterContext.Provider value={testRouter}>
        <ToastProvider>
          <AuthProvider>{node}</AuthProvider>
        </ToastProvider>
      </AppRouterContext.Provider>
    </IntlProvider>,
  );
}

const testRouter = {
  back() {},
  forward() {},
  prefetch() {},
  push() {},
  refresh() {},
  replace() {},
};

function installMessagesDom() {
  const dom = installDom();
  Object.defineProperty(globalThis, "requestAnimationFrame", {
    configurable: true,
    writable: true,
    value: dom.window.requestAnimationFrame,
  });
  Object.defineProperty(globalThis, "cancelAnimationFrame", {
    configurable: true,
    writable: true,
    value: dom.window.cancelAnimationFrame,
  });
}

function installApiMocks(options: MockOptions = {}) {
  const calls: { get: ApiCall[]; post: ApiCall[] } = { get: [], post: [] };

  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push({ path });
    if (path === "/api/v1/auth/me") {
      return { user } as T;
    }
    if (path === "/api/v1/notifications/unread-count") {
      return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } } as T;
    }
    if (path === "/api/v1/notifications") {
      return { notifications: options.notifications ?? [] } as T;
    }
    if (path === "/api/v1/messages") {
      return { conversations: [conversation], page: 1, page_size: 20 } as T;
    }
    if (path === "/api/v1/messages/42") {
      return {
        messages: options.detailMessages ?? [
          { id: 10, sender_id: 2, text: "hello", body: "hello", created_at: "2026-06-30T12:01:00Z" },
        ],
        total: 1,
      } as T;
    }
    throw new Error(`unexpected api.get path ${path}`);
  }) as typeof api.get;

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (options.postError) {
      throw options.postError;
    }
    if (options.postPromise) {
      return (await options.postPromise) as T;
    }
    if (path === "/api/v1/messages") {
      return {
        message: { id: 11, sender_id: 1, text: "hello back", body: "hello back", created_at: "2026-06-30T12:03:00Z" },
      } as T;
    }
    throw new Error(`unexpected api.post path ${path}`);
  }) as typeof api.post;

  api.patch = (async <T,>(path: string, body?: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (path.startsWith("/api/v1/notifications/")) {
      return {} as T;
    }
    throw new Error(`unexpected api.patch path ${path}`);
  }) as typeof api.patch;

  return calls;
}

function setValidAccessToken() {
  const payload = Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 })).toString("base64");
  setAccessToken(`test.${payload}.signature`);
}

async function findReplyInput(container: HTMLElement): Promise<HTMLTextAreaElement> {
  await waitFor(() => {
    assert.ok(container.querySelector('textarea[placeholder="Type a message..."]'), "reply input should render");
  });
  return container.querySelector('textarea[placeholder="Type a message..."]') as HTMLTextAreaElement;
}
