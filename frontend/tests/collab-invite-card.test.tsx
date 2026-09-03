import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import { AppRouterContext } from "next/dist/shared/lib/app-router-context.shared-runtime";

import { AuthProvider } from "@/contexts/AuthContext";
import { ApiRequestError, api, setAccessToken } from "@/lib/api";
import { ChatWindow } from "@/components/social/ChatWindow";
import { CollabInviteCard } from "@/components/social/CollabInviteCard";
import { ToastProvider } from "@/components/ui/Toast";

import { act, cleanup, fireEvent, installDom, renderWithIntl, waitFor } from "./runtime-test-helpers";

const originalGet = api.get;
const originalPost = api.post;

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
    id: 20,
    text: "Collaboration invite",
    sender_id: 2,
    created_at: "2026-06-30T12:10:00Z",
  },
  unread_count: 1,
  updated_at: "2026-06-30T12:10:00Z",
};

const inviteMetadata = {
  invite_id: 7,
  content_id: 601,
  content_title: "Star Dust fanwork",
  inviter_id: 2,
  inviter_username: "bob",
};

const pendingInvite = {
  id: 7,
  status: "pending" as const,
  contentId: 601,
  contentTitle: "Star Dust fanwork",
  inviterUsername: "bob",
};

const intlMessages = {
  common: {
    close: "Close",
    back: "Back",
    loading: "Loading...",
  },
  collabInviteCard: {
    type: "Collaboration invite",
    invitation: "{inviter} invites you to co-create “{title}”",
    status: {
      pending: "Pending",
      pendingSender: "Waiting for the invitee",
      accepted: "Accepted",
      declined: "Declined",
      expired: "Expired",
    },
    actions: {
      accept: "Accept",
      decline: "Decline",
    },
    errors: {
      failed: "Could not update the invitation. Please try again.",
    },
    summary: {
      invalid: "You received a collaboration invitation.",
    },
    a11y: {
      title: "Collaboration invitation for “{title}”",
      accept: "Accept collaboration invite for “{title}”",
      decline: "Decline collaboration invite for “{title}”",
    },
  },
  messages: {
    chat: {
      selectConversation: "Select a conversation",
      recipientUnavailable: "Recipient unavailable",
      retry: "Retry messages",
      emptyTitle: "No messages yet",
      emptyDescription: "Send a message to start this conversation.",
      unsupportedMessage: "This message type is not supported yet.",
      inputLabel: "Message",
      inputPlaceholder: "Type a message...",
      send: "Send message",
      sending: "Sending",
      timeUnknown: "Time unavailable",
      collabInviteSummary: "Collaboration invitation",
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
    error: {
      conversations: "Could not load conversations.",
      chat: "Could not load this conversation.",
      send: "Could not send the message.",
    },
    a11y: {
      conversationList: "Conversations",
      messageList: "Conversation messages",
    },
  },
};

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.post = originalPost;
  setAccessToken(null);
});

test("pending invite card shows accept and decline buttons with status text and group semantics", async () => {
  installCardDom();
  const view = renderCard(
    <CollabInviteCard
      invite={pendingInvite}
      isCurrentUserInvitee
      onAccept={async () => {}}
      onDecline={async () => {}}
    />,
  );

  const card = await waitFor(() => {
    const element = view.container.querySelector('[role="group"]');
    assert.ok(element, "card should use role=group");
    return element as HTMLElement;
  });
  assert.ok(card.getAttribute("aria-labelledby"), "card should be labelled via aria-labelledby");
  assert.ok(view.getByText(intlMessages.collabInviteCard.status.pending));
  assert.ok(view.getByRole("button", { name: `Accept collaboration invite for “${pendingInvite.contentTitle}”` }));
  assert.ok(view.getByRole("button", { name: `Decline collaboration invite for “${pendingInvite.contentTitle}”` }));
  assert.ok(view.getByText(`bob invites you to co-create “${pendingInvite.contentTitle}”`));
});

test("accepted invite card is read-only", async () => {
  installCardDom();
  const view = renderCard(
    <CollabInviteCard
      invite={{ ...pendingInvite, status: "accepted" }}
      isCurrentUserInvitee
      onAccept={async () => {}}
      onDecline={async () => {}}
    />,
  );

  await waitFor(() => assert.ok(view.getByText(intlMessages.collabInviteCard.status.accepted)));
  assert.equal(view.queryByRole("button", { name: /Accept collaboration invite/ }), null);
  assert.equal(view.queryByRole("button", { name: /Decline collaboration invite/ }), null);
  assert.ok(view.getByRole("link", { name: pendingInvite.contentTitle }), "content link stays available");
});

test("declined invite card is read-only", async () => {
  installCardDom();
  const view = renderCard(
    <CollabInviteCard
      invite={{ ...pendingInvite, status: "declined" }}
      isCurrentUserInvitee
      onAccept={async () => {}}
      onDecline={async () => {}}
    />,
  );

  await waitFor(() => assert.ok(view.getByText(intlMessages.collabInviteCard.status.declined)));
  assert.equal(view.queryByRole("button", { name: /Accept collaboration invite/ }), null);
  assert.equal(view.queryByRole("button", { name: /Decline collaboration invite/ }), null);
});

test("expired invite card is read-only and muted", async () => {
  installCardDom();
  const view = renderCard(
    <CollabInviteCard
      invite={{ ...pendingInvite, status: "expired" }}
      isCurrentUserInvitee
      onAccept={async () => {}}
      onDecline={async () => {}}
    />,
  );

  await waitFor(() => assert.ok(view.getByText(intlMessages.collabInviteCard.status.expired)));
  assert.equal(view.queryByRole("button", { name: /Accept collaboration invite/ }), null);
  assert.equal(view.queryByRole("button", { name: /Decline collaboration invite/ }), null);
  assert.ok(
    view.container.querySelector('[role="group"]')?.className.includes("bg-canvas-subtle"),
    "expired card should use the muted canvas background",
  );
});

test("non-invitee pending card shows waiting status without action buttons", async () => {
  installCardDom();
  const view = renderCard(
    <CollabInviteCard
      invite={pendingInvite}
      isCurrentUserInvitee={false}
      onAccept={async () => {}}
      onDecline={async () => {}}
    />,
  );

  await waitFor(() => assert.ok(view.getByText(intlMessages.collabInviteCard.status.pendingSender)));
  assert.equal(view.queryByRole("button", { name: /Accept collaboration invite/ }), null);
  assert.equal(view.queryByRole("button", { name: /Decline collaboration invite/ }), null);
});

test("accept button posts to the accept endpoint and applies the returned invite status", async () => {
  installCardDom();
  const calls = installCardApiMocks({ acceptResult: "accepted" });
  const acceptedIds: number[] = [];
  const view = renderCard(
    <CollabInviteCard
      invite={pendingInvite}
      isCurrentUserInvitee
      onAccept={async (inviteId) => {
        acceptedIds.push(inviteId);
      }}
      onDecline={async () => {}}
    />,
  );

  fireEvent.click(view.getByRole("button", { name: `Accept collaboration invite for “${pendingInvite.contentTitle}”` }));

  await waitFor(() => {
    assert.ok(
      calls.post.some((call) => call.path === "/api/v1/collab-invites/7/accept"),
      `expected POST accept, got ${JSON.stringify(calls.post)}`,
    );
    assert.ok(view.getByText(intlMessages.collabInviteCard.status.accepted), "status should come from the returned invite");
  });
  assert.equal(view.queryByRole("button", { name: /Accept collaboration invite/ }), null);
  assert.equal(view.queryByRole("button", { name: /Decline collaboration invite/ }), null);
  assert.deepEqual(acceptedIds, [7]);
});

test("decline button posts to the decline endpoint and applies the returned invite status", async () => {
  installCardDom();
  const calls = installCardApiMocks({ declineResult: "declined" });
  const declinedIds: number[] = [];
  const view = renderCard(
    <CollabInviteCard
      invite={pendingInvite}
      isCurrentUserInvitee
      onAccept={async () => {}}
      onDecline={async (inviteId) => {
        declinedIds.push(inviteId);
      }}
    />,
  );

  fireEvent.click(view.getByRole("button", { name: `Decline collaboration invite for “${pendingInvite.contentTitle}”` }));

  await waitFor(() => {
    assert.ok(
      calls.post.some((call) => call.path === "/api/v1/collab-invites/7/decline"),
      `expected POST decline, got ${JSON.stringify(calls.post)}`,
    );
    assert.ok(view.getByText(intlMessages.collabInviteCard.status.declined));
  });
  assert.deepEqual(declinedIds, [7]);
});

test("accept failure keeps pending state and shows inline error plus toast", async () => {
  installCardDom();
  installCardApiMocks({ acceptError: new ApiRequestError("INVITE_ALREADY_RESPONDED", "already responded", 409) });
  const view = renderCard(
    <CollabInviteCard
      invite={pendingInvite}
      isCurrentUserInvitee
      onAccept={async () => {}}
      onDecline={async () => {}}
    />,
  );

  fireEvent.click(view.getByRole("button", { name: `Accept collaboration invite for “${pendingInvite.contentTitle}”` }));

  await waitFor(() => {
    assert.ok(
      view.container.querySelector('[role="alert"]')?.textContent?.includes(intlMessages.collabInviteCard.errors.failed),
      "inline error should render",
    );
    assert.ok(document.body.textContent?.includes(intlMessages.collabInviteCard.errors.failed), "toast should render");
  });
  assert.ok(view.getByRole("button", { name: `Accept collaboration invite for “${pendingInvite.contentTitle}”` }), "card should stay actionable");
  assert.ok(view.getByText(intlMessages.collabInviteCard.status.pending), "status should stay pending after failure");
});

test("loading sets aria-busy on the card and disables the other action button", async () => {
  installCardDom();
  let resolveAccept: (value: unknown) => void = () => {};
  const pendingAccept = new Promise((resolve) => {
    resolveAccept = resolve;
  });
  installCardApiMocks({ acceptPromise: pendingAccept });
  const view = renderCard(
    <CollabInviteCard
      invite={pendingInvite}
      isCurrentUserInvitee
      onAccept={async () => {}}
      onDecline={async () => {}}
    />,
  );

  fireEvent.click(view.getByRole("button", { name: `Accept collaboration invite for “${pendingInvite.contentTitle}”` }));

  await waitFor(() => {
    const card = view.container.querySelector('[role="group"]');
    assert.equal(card?.getAttribute("aria-busy"), "true", "card should be busy while saving");
    const decline = view.getByRole("button", { name: `Decline collaboration invite for “${pendingInvite.contentTitle}”` }) as HTMLButtonElement;
    assert.equal(decline.disabled, true, "other action should be disabled while busy");
  });

  await act(async () => {
    resolveAccept({ invite: { id: 7, status: "accepted" } });
    await pendingAccept;
  });

  await waitFor(() => {
    assert.equal(view.container.querySelector('[role="group"]')?.getAttribute("aria-busy"), "false");
  });
});

test("action buttons have aria-labels with the content title and 44px touch targets on mobile", async () => {
  installCardDom();
  const view = renderCard(
    <CollabInviteCard
      invite={pendingInvite}
      isCurrentUserInvitee
      onAccept={async () => {}}
      onDecline={async () => {}}
    />,
  );

  await waitFor(() => {
    const accept = view.getByRole("button", { name: `Accept collaboration invite for “${pendingInvite.contentTitle}”` });
    const decline = view.getByRole("button", { name: `Decline collaboration invite for “${pendingInvite.contentTitle}”` });
    assert.ok(accept.className.includes("min-h-11"), "mobile touch target should be at least 44px");
    assert.ok(decline.className.includes("min-h-11"), "mobile touch target should be at least 44px");
  });
});

test("ChatWindow renders the invite card for collab_invite messages and keeps text bubbles", async () => {
  installCardDom();
  installChatWindowApiMocks([
    { id: 20, sender_id: 2, msg_type: "collab_invite", text: "Collaboration invite", body: "Collaboration invite", metadata: inviteMetadata, created_at: "2026-06-30T12:10:00Z" },
    { id: 10, sender_id: 2, text: "hello", body: "hello", created_at: "2026-06-30T12:01:00Z" },
  ]);

  const view = renderChatWindow();

  await waitFor(() => {
    assert.ok(view.getByRole("button", { name: `Accept collaboration invite for “${pendingInvite.contentTitle}”` }), "invite card should render in the flow");
    assert.ok(view.getByText("hello"), "normal text message should stay a bubble");
  });
  assert.equal(view.queryByText(intlMessages.messages.chat.unsupportedMessage), null, "collab invite must not fall back to the unsupported message");
});

test("ChatWindow falls back to a safe summary for collab_invite messages with invalid metadata", async () => {
  installCardDom();
  installChatWindowApiMocks([
    { id: 21, sender_id: 2, msg_type: "collab_invite", text: "Collaboration invite", body: "Collaboration invite", metadata: {}, created_at: "2026-06-30T12:11:00Z" },
  ]);

  const view = renderChatWindow();

  await waitFor(() => assert.ok(view.getByText(intlMessages.collabInviteCard.summary.invalid)));
  assert.equal(view.queryByRole("button", { name: /Accept collaboration invite/ }), null, "no actions for invalid metadata");
});

function renderCard(node: React.ReactNode) {
  return renderWithIntl(
    <IntlProvider locale="en" messages={intlMessages}>
      <AppRouterContext.Provider value={testRouter}>
        <ToastProvider>{node}</ToastProvider>
      </AppRouterContext.Provider>
    </IntlProvider>,
  );
}

function renderChatWindow() {
  setValidAccessToken();
  return renderWithIntl(
    <IntlProvider locale="en" messages={intlMessages}>
      <AppRouterContext.Provider value={testRouter}>
        <ToastProvider>
          <AuthProvider>
            <ChatWindow conversation={conversation} />
          </AuthProvider>
        </ToastProvider>
      </AppRouterContext.Provider>
    </IntlProvider>,
  );
}

const testRouter = {
  back() {},
  forward() {},
  prefetch() {},
  bfcacheId: "test-bfcache",
  push() {},
  refresh() {},
  replace() {},
};

function installCardDom() {
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

type CardMockOptions = {
  acceptResult?: "accepted";
  declineResult?: "declined";
  acceptError?: Error;
  acceptPromise?: Promise<unknown>;
};

function installCardApiMocks(options: CardMockOptions = {}) {
  const calls: { post: Array<{ path: string; body?: unknown }> } = { post: [] };

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if (options.acceptPromise) {
      return (await options.acceptPromise) as T;
    }
    if (options.acceptError && path.endsWith("/accept")) {
      throw options.acceptError;
    }
    if (path.endsWith("/accept")) {
      return { invite: { id: 7, status: options.acceptResult ?? "accepted" } } as T;
    }
    if (path.endsWith("/decline")) {
      return { invite: { id: 7, status: options.declineResult ?? "declined" } } as T;
    }
    throw new Error(`unexpected api.post path ${path}`);
  }) as typeof api.post;

  return calls;
}

function installChatWindowApiMocks(detailMessages: Array<Record<string, unknown>>) {
  api.get = (async <T,>(path: string): Promise<T> => {
    if (path === "/api/v1/auth/me") {
      return { user } as T;
    }
    if (path === "/api/v1/notifications/unread-count") {
      return { unread_counts: { total: 0, reply: 0, like: 0, system: 0, pr: 0, follow: 0 } } as T;
    }
    if (path === "/api/v1/messages/42") {
      return { messages: detailMessages, total: detailMessages.length } as T;
    }
    throw new Error(`unexpected api.get path ${path}`);
  }) as typeof api.get;
}

function setValidAccessToken() {
  const payload = Buffer.from(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 })).toString("base64");
  setAccessToken(`test.${payload}.signature`);
}
