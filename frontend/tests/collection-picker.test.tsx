import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";
import React from "react";
import { IntlProvider } from "use-intl";

import { api } from "@/lib/api";
import {
  addCollectionItem,
  createCollection,
  deleteCollection,
  getCollection,
  listCollections,
  removeCollectionItem,
  updateCollection,
} from "@/lib/collections";
import { cleanup, fireEvent, installDom, waitFor } from "./runtime-test-helpers";
import { render } from "@testing-library/react";

type ApiCall = {
  path: string;
  body?: unknown;
};

const originalGet = api.get;
const originalPost = api.post;
const originalPut = api.put;
const originalDelete = api.delete;
const originalConsoleError = console.error;

test.beforeEach(() => {
  console.error = (...args: unknown[]) => {
    if (
      args.some(
        (arg) =>
          typeof arg === "object" &&
          arg !== null &&
          "code" in arg &&
          (arg as { code?: string }).code === "ENVIRONMENT_FALLBACK",
      )
    ) {
      return;
    }
    originalConsoleError(...args);
  };
});

test.afterEach(() => {
  cleanup();
  api.get = originalGet;
  api.post = originalPost;
  api.put = originalPut;
  api.delete = originalDelete;
  console.error = originalConsoleError;
});

test("listCollections calls the picker list endpoint and normalizes item flags", async () => {
  const calls = installApiMocks({
    getResponse: {
      items: [
        { id: 1, title: "Favorites", contains_item: true, item_id: 31 },
        { id: 2, title: "Later", item_id: "not-a-number" },
      ],
      total: 2,
    },
  });

  const result = await listCollections({ zone: "original", contentItemId: 42 });

  assert.equal(calls.get.at(-1)?.path, "/api/v1/collections?zone=original&content_item_id=42");
  assert.equal(result.total, 2);
  assert.equal(result.collections[0]?.contains_item, true);
  assert.equal(result.collections[0]?.item_id, 31);
  assert.equal(result.collections[1]?.contains_item, false);
  assert.equal("item_id" in result.collections[1]!, false);
  assert.equal("items" in result, false);
});

test("listCollections serializes owner id and handles collection aliases", async () => {
  const calls = installApiMocks({
    getResponse: {
      collections: [{ id: 4, title: "Public shelf", contains_item: 1, item_id: 91 }],
      total: 1,
    },
  });

  const result = await listCollections({ ownerId: 20 });

  assert.equal(calls.get.at(-1)?.path, "/api/v1/collections?owner_id=20");
  assert.equal(result.collections[0]?.contains_item, true);
  assert.equal(result.collections[0]?.item_id, 91);
});

test("createCollection posts collection input", async () => {
  const calls = installApiMocks({ postResponse: { collection: { id: 7, title: "Reference shelf" } } });
  const input = {
    title: "Reference shelf",
    description: "Research material",
    zone: "original",
    is_public: false,
  };

  const result = await createCollection(input);

  assert.deepEqual(calls.post.at(-1), {
    path: "/api/v1/collections",
    body: input,
  });
  assert.equal(result.id, 7);
});

test("getCollection calls the detail endpoint with pagination and content type", async () => {
  const calls = installApiMocks();

  await getCollection(9, { page: 2, pageSize: 24, contentType: "video" });

  assert.equal(calls.get.at(-1)?.path, "/api/v1/collections/9?page=2&page_size=24&content_type=video");
});

test("addCollectionItem posts the content item and optional note", async () => {
  const calls = installApiMocks({ postResponse: { item: { id: 51, note: "opening scene" } } });

  const result = await addCollectionItem(9, 42, "opening scene");

  assert.deepEqual(calls.post.at(-1), {
    path: "/api/v1/collections/9/items",
    body: { content_item_id: 42, note: "opening scene" },
  });
  assert.equal(result.id, 51);
});

test("removeCollectionItem deletes the collection item endpoint", async () => {
  const calls = installApiMocks();

  await removeCollectionItem(9, 31);

  assert.equal(calls.delete.at(-1)?.path, "/api/v1/collections/9/items/31");
});

test("updateCollection puts a collection patch", async () => {
  const calls = installApiMocks({ putResponse: { collection: { id: 9, title: "Updated shelf" } } });
  const patch = { title: "Updated shelf", is_public: true, sort_order: 3 };

  const result = await updateCollection(9, patch);

  assert.deepEqual(calls.put.at(-1), {
    path: "/api/v1/collections/9",
    body: patch,
  });
  assert.equal(result.title, "Updated shelf");
});

test("deleteCollection deletes the collection endpoint", async () => {
  const calls = installApiMocks();

  await deleteCollection(9);

  assert.equal(calls.delete.at(-1)?.path, "/api/v1/collections/9");
});

test("CollectionPicker lists same-zone collections with content item state from the list API", async () => {
  installDom();
  const calls = installApiMocks({
    getResponse: {
      items: [
        collectionSummary(1, "Original default", "original", false),
        collectionSummary(2, "Already in", "original", true, 991),
        { ...collectionSummary(3, "Fanwork shelf", "fanwork", false), zone: "fanwork" },
        { ...collectionSummary(4, "Malformed shelf", "original", false), zone: undefined },
      ],
      total: 4,
    },
  });

  const view = renderPicker();

  await waitFor(() => assert.equal(calls.get.at(-1)?.path, "/api/v1/collections?zone=original&content_item_id=42"));
  assert.ok(view.getByText("Original default"));
  assert.ok(view.getByText("Already in"));
  assert.ok(!view.queryByText("Fanwork shelf"));
  assert.ok(!view.queryByText("Malformed shelf"));
  assert.ok(view.getByText("Added"));
  assert.equal(calls.post.length, 0, "already-added state must not be probed with a duplicate add request");
});

test("CollectionPicker removes already-added content using backend item_id", async () => {
  installDom();
  const calls = installApiMocks({
    getResponse: {
      items: [collectionSummary(2, "Already in", "original", true, 991)],
      total: 1,
    },
  });
  const view = renderPicker();

  fireEvent.click(await view.findByRole("button", { name: "Remove from Already in" }));

  await waitFor(() => assert.equal(calls.delete.at(-1)?.path, "/api/v1/collections/2/items/991"));
  assert.equal(calls.post.length, 0);
});

test("CollectionPicker creates a collection inline and adds the current item", async () => {
  installDom();
  const calls = installApiMocks({
    getResponse: { items: [], total: 0 },
    postResponseForPath(path) {
      if (path === "/api/v1/collections") {
        return { collection: { id: 77, title: "Research", zone: "original", is_public: false, is_default: false, item_count: 0 } };
      }
      return { item: { id: 123, collection_id: 77, content_item_id: 42 } };
    },
  });
  const view = renderPicker();

  fireEvent.click(await view.findByRole("button", { name: "New collection" }));
  setNativeInputValue(view.getByLabelText("Collection title"), "Research");
  fireEvent.click(view.getByRole("button", { name: "Create and add" }));

  await waitFor(() => {
    assert.deepEqual(calls.post.at(-2), {
      path: "/api/v1/collections",
      body: { title: "Research", description: "", zone: "original", is_public: false },
    });
    assert.deepEqual(calls.post.at(-1), {
      path: "/api/v1/collections/77/items",
      body: { content_item_id: 42 },
    });
  });
});

test("CollectionPicker shows search only when there are ten or more collections", async () => {
  installDom();
  installApiMocks({
    getResponse: {
      items: Array.from({ length: 10 }, (_, index) =>
        collectionSummary(index + 1, `Shelf ${index + 1}`, "original", false),
      ),
      total: 10,
    },
  });

  const view = renderPicker();

  assert.ok(await view.findByRole("searchbox", { name: "Search collections" }));
});

test("ContentDetail opens CollectionPicker instead of keeping the legacy favorite toggle as primary action", () => {
  const source = fs.readFileSync(new URL("../components/content/ContentDetail.tsx", import.meta.url), "utf8");

  assert.match(source, /CollectionPicker/);
  assert.match(source, /setCollectionPickerOpen\(true\)/);
  assert.doesNotMatch(source, /onClick=\{\(\) => void toggleFavorite\(\)\}/);
});

test("Studio favorites page uses CollectionCard and collection APIs instead of the legacy favorites list", () => {
  const source = fs.readFileSync(new URL("../app/(protected)/studio/favorites/page.tsx", import.meta.url), "utf8");

  assert.match(source, /CollectionCard/);
  assert.match(source, /listCollections/);
  assert.match(source, /createCollection/);
  assert.match(source, /updateCollection/);
  assert.match(source, /deleteCollection/);
  assert.doesNotMatch(source, /\/api\/v1\/users\/\$\{user\.id\}\/favorites/);
});

test("new Task 7 code does not import the legacy add-to-collection modal", () => {
  const ownedFiles = [
    "../components/content/CollectionPicker.tsx",
    "../components/content/CollectionCard.tsx",
    "../components/content/ContentDetail.tsx",
    "../components/content/ContentDetailClient.tsx",
    "../app/(protected)/studio/favorites/page.tsx",
  ];

  for (const file of ownedFiles) {
    const source = fs.existsSync(new URL(file, import.meta.url))
      ? fs.readFileSync(new URL(file, import.meta.url), "utf8")
      : "";
    assert.doesNotMatch(source, /AddToCollection|add-to-collection|LegacyAddToCollection|CollectionModal/);
  }
});

function installApiMocks(
  options: {
    getResponse?: unknown;
    postResponse?: unknown;
    putResponse?: unknown;
    postResponseForPath?: (path: string, body: unknown) => unknown;
  } = {},
) {
  const calls: { get: ApiCall[]; post: ApiCall[]; put: ApiCall[]; delete: ApiCall[] } = {
    get: [],
    post: [],
    put: [],
    delete: [],
  };

  api.get = (async <T,>(path: string): Promise<T> => {
    calls.get.push({ path });
    return (options.getResponse ?? { collection: { id: 9 }, collections: [] }) as T;
  }) as typeof api.get;

  api.post = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.post.push({ path, body });
    if ("postResponseForPath" in options && typeof options.postResponseForPath === "function") {
      return options.postResponseForPath(path, body) as T;
    }
    return (options.postResponse ?? { id: 1 }) as T;
  }) as typeof api.post;

  api.put = (async <T,>(path: string, body: unknown): Promise<T> => {
    calls.put.push({ path, body });
    return (options.putResponse ?? { id: 1 }) as T;
  }) as typeof api.put;

  api.delete = (async <T,>(path: string): Promise<T> => {
    calls.delete.push({ path });
    return undefined as T;
  }) as typeof api.delete;

  return calls;
}

function collectionSummary(id: number, title: string, zone: "original" | "fanwork", containsItem: boolean, itemId?: number) {
  return {
    id,
    title,
    zone,
    description: "",
    is_public: false,
    is_default: id === 1,
    item_count: containsItem ? 1 : 0,
    contains_item: containsItem,
    ...(itemId !== undefined ? { item_id: itemId } : {}),
  };
}

function renderPicker() {
  return render(
    <IntlProvider locale="en" messages={pickerMessages}>
      <CollectionPickerHarness />
    </IntlProvider>,
  );
}

function setNativeInputValue(element: HTMLElement, value: string) {
  const input = element as HTMLInputElement;
  const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value")?.set;
  setter?.call(input, value);
  fireEvent.input(input, { bubbles: true });
}

function CollectionPickerHarness() {
  const [open, setOpen] = React.useState(true);
  const [changedCount, setChangedCount] = React.useState(0);
  const [Picker, setPicker] = React.useState<React.ComponentType<{
    contentId: number;
    contentTitle: string;
    zone: "original" | "fanwork";
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onChanged?: () => void;
  }> | null>(null);

  React.useEffect(() => {
    void import("@/components/content/CollectionPicker").then((module) => {
      setPicker(() => module.CollectionPicker);
    });
  }, []);

  if (!Picker) return null;

  return (
    <>
      <Picker
        contentId={42}
        contentTitle="Current content"
        zone="original"
        open={open}
        onOpenChange={setOpen}
        onChanged={() => setChangedCount((count) => count + 1)}
      />
      <output aria-label="changed count">{changedCount}</output>
    </>
  );
}

const pickerMessages = {
  common: {
    cancel: "Cancel",
    close: "Close",
    processing: "Processing",
    operationFailed: "Operation failed",
    retry: "Retry",
    edit: "Edit",
    delete: "Delete",
    view: "View",
  },
  collections: {
    picker: {
      title: "Add to collection",
      description: "Save {title} to an {zone} collection.",
      zone: {
        original: "original",
        fanwork: "fan creation",
      },
      search: {
        label: "Search collections",
        placeholder: "Search collections",
        empty: "No matching collections",
      },
      actions: {
        add: "Add",
        remove: "Remove",
        removeFrom: "Remove from {title}",
        new: "New collection",
        close: "Close",
      },
      create: {
        title: "Collection title",
        titlePlaceholder: "Collection name",
        description: "Description",
        descriptionPlaceholder: "Optional note",
        isPublic: "Public collection",
        submit: "Create and add",
      },
      states: {
        loading: "Loading collections",
        empty: "No collections yet",
        added: "Added",
        private: "Private",
        public: "Public",
        default: "Default",
        itemCount: "{count} items",
      },
      errors: {
        load: "Failed to load collections",
        create: "Failed to create collection",
        add: "Failed to add",
        remove: "Failed to remove",
      },
      toast: {
        added: "Added to collection",
        removed: "Removed from collection",
        created: "Collection created",
      },
    },
    card: {
      public: "Public",
      private: "Private",
      default: "Default",
      itemCount: "{count} items",
      edit: "Edit {title}",
      delete: "Delete {title}",
      deleteDisabled: "Default collections cannot be deleted",
    },
  },
};
