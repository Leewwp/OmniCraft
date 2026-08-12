"use client";

import { useState } from "react";
import { CollabUserPicker } from "@/components/content/CollabUserPicker";

interface DemoUser {
  id: number;
  username: string;
  avatarUrl?: string;
}

export default function CollabPickerDemoPage() {
  const [selectedUsers, setSelectedUsers] = useState<DemoUser[]>([]);

  return (
    <div className="mx-auto w-full max-w-[960px] p-6">
      <h1 className="mb-4 text-lg font-semibold">CollabUserPicker 演示</h1>
      <div className="space-y-3">
        <CollabUserPicker selectedUsers={selectedUsers} maxSelected={5} onChange={setSelectedUsers} />
      </div>
    </div>
  );
}
