import test from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { IntlProvider } from "use-intl";
import enMessages from "@/messages/en.json";

import { api } from "@/lib/api";
import { cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";
import { CollabUserPicker } from "@/components/content/CollabUserPicker";

const originalGet = api.get;

interface SelectedUser {
  id: number;
  username: string;
  avatarUrl?: string;
}

type PickerApiCall = {
  url: string;
  resolve: (data: unknown) => void;
  reject: (error: Error) => void;
};

let apiCalls: PickerApiCall[] = [];
let failNextSearch = false;

function installSearchMock() {
  apiCalls = [];
  failNextSearch = false;
  api.get = <T,>(path: string): Promise<T> =>
    new Promise<T>((resolve, reject) => {
      apiCalls.push({ url: path, resolve: (data: unknown) => resolve(data as T), reject });
      if (failNextSearch) {
        failNextSearch = false;
        reject(new Error("network error"));
      }
    });
}

function searchUser(id: number, username: string, avatarUrl?: string): Record<string, unknown> {
  return { id, username, ...(avatarUrl !== undefined ? { avatar_url: avatarUrl } : {}) };
}

function PickerHarness({
  initial = [],
  maxSelected,
  disabled,
  onChange,
}: {
  initial?: SelectedUser[];
  maxSelected: number;
  disabled?: boolean;
  onChange?: (users: SelectedUser[]) => void;
}) {
  const [selectedUsers, setSelectedUsers] = React.useState<SelectedUser[]>(initial);
  return (
    <IntlProvider locale="en" messages={enMessages}>
      <CollabUserPicker
        selectedUsers={selectedUsers}
        maxSelected={maxSelected}
        disabled={disabled}
        onChange={(users) => {
          setSelectedUsers(users);
          onChange?.(users);
        }}
      />
    </IntlProvider>
  );
}

async function renderPicker(
  props: {
    initial?: SelectedUser[];
    maxSelected?: number;
    disabled?: boolean;
    onChange?: (users: SelectedUser[]) => void;
  } = {},
) {
  const { render } = await import("@testing-library/react");
  const selections: SelectedUser[][] = [];
  const view = render(
    <PickerHarness
      initial={props.initial}
      maxSelected={props.maxSelected ?? 5}
      disabled={props.disabled}
      onChange={(users) => {
        selections.push(users);
        props.onChange?.(users);
      }}
    />,
  );
  return { view, selections };
}

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
});

test("picker searches users by username and calls exactly GET /api/v1/users/search?q=<query>&limit=8", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker();

  const input = view.getByRole("combobox", { name: enMessages.collabUserPicker.label });
  fireEvent.change(input, { target: { value: "Lumi" } });

  await waitFor(() => assert.equal(apiCalls.length, 1));
  assert.equal(apiCalls[0].url, "/api/v1/users/search?q=Lumi&limit=8");

  apiCalls[0].resolve({ users: [searchUser(1, "Luminary", "https://cdn.example/1.png")], total: 1 });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Luminary/ })));
});

test("picker result normalization keeps only id, username, and avatarUrl", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker();

  fireEvent.change(view.getByRole("combobox"), { target: { value: "Lumi" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({
    users: [searchUser(1, "Luminary", "https://cdn.example/1.png")].map((user) => ({
      ...user,
      reputation: 10,
      role: "creator",
    })),
    total: 1,
  });
  const option = await waitFor(() => view.getByRole("option", { name: /Luminary/ }));
  fireEvent.click(option);

  await waitFor(() => assert.equal(selections.length, 1));
  assert.deepEqual(selections[0], [{ id: 1, username: "Luminary", avatarUrl: "https://cdn.example/1.png" }]);
  assert.equal("reputation" in selections[0][0]!, false, "reputation must not leak into the selection");
  assert.equal("role" in selections[0][0]!, false, "role must not leak into the selection");
});

test("picker drops results without numeric id or non-empty username", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker();

  fireEvent.change(view.getByRole("combobox"), { target: { value: "Mix" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({
    users: [
      searchUser(11, "Valid User"),
      { id: 12, username: "" },
      { id: 13 },
      { id: "14", username: "String Id" },
      { id: 15, username: "   " },
    ],
    total: 5,
  });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Valid User/ })));
  assert.equal(view.queryByRole("option", { name: /String Id/ }), null);
  assert.equal(view.queryByRole("option", { name: "" }), null);
});

test("selected collaborators can be removed", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({
    initial: [{ id: 1, username: "Luminary", avatarUrl: "https://cdn.example/1.png" }],
  });

  const removeLabel = enMessages.collabUserPicker.a11y.removeUser.replace("{username}", "Luminary");
  fireEvent.click(view.getByRole("button", { name: removeLabel }));

  await waitFor(() => assert.equal(selections.length, 1));
  assert.deepEqual(selections[0], []);
});

test("duplicate selected users cannot be added and render a localized disabled state", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({
    initial: [{ id: 1, username: "Luminary" }],
  });

  fireEvent.change(view.getByRole("combobox"), { target: { value: "Lumi" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  apiCalls[0].resolve({ users: [searchUser(1, "Luminary")], total: 1 });
  const option = await waitFor(() => view.getByRole("option", { name: /Luminary/ }));

  assert.equal(option.getAttribute("aria-selected"), "true", "already-selected option must carry aria-selected");
  assert.equal(option.getAttribute("aria-disabled"), "true", "already-selected option must be disabled");
  assert.ok(
    view.getByText(enMessages.collabUserPicker.duplicate.alreadySelected),
    "duplicate option explains the disabled state with localized text",
  );

  fireEvent.click(option);
  assert.equal(selections.length, 0, "clicking a duplicate must not change selection");
});

test("no more than maxSelected users can be selected and the limit message is localized", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({ maxSelected: 2 });

  for (const [index, username] of ["Alice", "Bob"].entries()) {
    const input = view.getByRole("combobox") as HTMLInputElement;
    fireEvent.change(input, { target: { value: username } });
    await waitFor(() => assert.equal(apiCalls.length, index + 1));
    apiCalls[index].resolve({ users: [searchUser(index + 1, username)], total: 1 });
    const option = await waitFor(() => view.getByRole("option", { name: new RegExp(username) }));
    fireEvent.click(option);
  }

  await waitFor(() => assert.equal(selections.length, 2));
  assert.deepEqual(selections[1], [
    { id: 1, username: "Alice" },
    { id: 2, username: "Bob" },
  ]);

  const maxReached = enMessages.collabUserPicker.selected.maxReached.replace("{max}", "2");
  assert.ok(view.getByText(maxReached), "limit message renders once the cap is reached");

  const input = view.getByRole("combobox") as HTMLInputElement;
  assert.equal(input.disabled, true, "search input is disabled when the cap is reached");
  fireEvent.change(input, { target: { value: "Carol" } });
  assert.equal(apiCalls.length, 2, "no search fires while the cap is reached");
});

test("CollabUserPicker receives the public value as maxSelected", async () => {
  installDom();
  installSearchMock();
  const { view, selections } = await renderPicker({ maxSelected: 5 });

  for (let index = 0; index < 5; index += 1) {
    const username = `User${index + 1}`;
    const input = view.getByRole("combobox") as HTMLInputElement;
    fireEvent.change(input, { target: { value: username } });
    await waitFor(() => assert.equal(apiCalls.length, index + 1));
    apiCalls[index].resolve({ users: [searchUser(index + 1, username)], total: 1 });
    const option = await waitFor(() => view.getByRole("option", { name: new RegExp(username) }));
    fireEvent.click(option);
  }

  await waitFor(() => assert.equal(selections.length, 5));
  assert.deepEqual(selections[4], Array.from({ length: 5 }, (_, index) => ({ id: index + 1, username: `User${index + 1}` })));
});

test("zero maxSelected keeps selection unavailable with localized explanation and no api calls", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({ maxSelected: 0 });

  const input = view.getByRole("combobox") as HTMLInputElement;
  assert.equal(input.disabled, true, "search input must be disabled when selection is unavailable");
  assert.ok(view.getByText(enMessages.collabUserPicker.disabled.unavailable), "unavailable state renders a localized explanation");

  fireEvent.change(input, { target: { value: "Lumi" } });
  assert.equal(apiCalls.length, 0, "no search api call may fire while selection is unavailable");
});

test("loading, empty, and error states render localized text", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker();
  const input = view.getByRole("combobox");

  fireEvent.change(input, { target: { value: "Wait" } });
  await waitFor(() => assert.equal(apiCalls.length, 1));
  assert.ok(view.getByText(enMessages.collabUserPicker.search.loading));
  apiCalls[0].resolve({ users: [], total: 0 });
  await waitFor(() => assert.ok(view.getByText(enMessages.collabUserPicker.search.empty)));

  failNextSearch = true;
  fireEvent.change(input, { target: { value: "Fail" } });
  await waitFor(() => assert.equal(apiCalls.length, 2));
  await waitFor(() => assert.ok(view.getByText(enMessages.collabUserPicker.error.searchFailed)));
  fireEvent.click(view.getByRole("button", { name: enMessages.collabUserPicker.error.retry }));
  await waitFor(() => assert.equal(apiCalls.length, 3));
  apiCalls[2].resolve({ users: [searchUser(9, "Recovered")], total: 1 });
  await waitFor(() => assert.ok(view.getByRole("option", { name: /Recovered/ })));
});

test("disabled picker disables the search input and chip removal", async () => {
  installDom();
  installSearchMock();
  const { view } = await renderPicker({
    initial: [{ id: 1, username: "Luminary" }],
    disabled: true,
  });

  const input = view.getByRole("combobox") as HTMLInputElement;
  assert.equal(input.disabled, true);
  const removeLabel = enMessages.collabUserPicker.a11y.removeUser.replace("{username}", "Luminary");
  const remove = view.getByRole("button", { name: removeLabel });
  assert.equal(remove.hasAttribute("disabled"), true);
});
