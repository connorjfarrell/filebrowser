<template>
  <div class="card-content">
    <p v-if="isNew">
      <label for="group-name">{{ $t("access.groupName") }}</label>
      <input
        id="group-name"
        class="input"
        type="text"
        v-model.trim="name"
        v-focus
        @keyup.enter="submit"
      />
    </p>
    <p v-else>
      <label>{{ $t("access.groupName") }}</label>
      <input class="input" type="text" :value="name" disabled />
    </p>
    <label>{{ membersLabel }}</label>
    <input
      class="input"
      type="text"
      v-model="filter"
      :placeholder="$t('access.filterUsers')"
      :aria-label="$t('access.filterUsers')"
    />
    <div class="member-list">
      <label v-for="username in visibleUsers" :key="username" class="member-item">
        <input type="checkbox" :value="username" v-model="selected" />
        {{ username }}
      </label>
    </div>
  </div>

  <div class="card-actions">
    <button
      type="button"
      class="button button--flat button--grey"
      @click="closeTopPrompt"
      :aria-label="$t('general.cancel')"
    >
      {{ $t("general.cancel") }}
    </button>
    <button
      type="button"
      class="button button--flat"
      :disabled="!name || saving"
      @click="submit"
      :aria-label="$t('general.save')"
    >
      {{ $t("general.save") }}
    </button>
  </div>
</template>

<script>
import { accessApi, usersApi } from "@/api";
import { mutations } from "@/store";
import { notify } from "@/notify";
import { eventBus } from "@/store/eventBus";

export default {
  name: "group-edit",
  props: {
    group: { type: String, default: "" },
    members: { type: Array, default: () => [] },
  },
  data() {
    return {
      name: this.group,
      selected: [...this.members],
      allUsers: [],
      filter: "",
      saving: false,
    };
  },
  async created() {
    try {
      const users = await usersApi.getAllUsers();
      const names = new Set(users.map((u) => u.username));
      // Keep members that no longer map to a local user (e.g. OIDC-only) visible.
      this.members.forEach((m) => names.add(m));
      this.allUsers = [...names].sort((a, b) => a.localeCompare(b));
    } catch (e) {
      notify.showError(e);
    }
  },
  computed: {
    membersLabel() {
      return `${this.$t("access.groupMembers")} (${this.selected.length})`;
    },
    isNew() {
      return !this.group;
    },
    visibleUsers() {
      const q = this.filter.toLowerCase();
      return this.allUsers.filter((u) => u.toLowerCase().includes(q));
    },
  },
  methods: {
    closeTopPrompt() {
      mutations.closeTopPrompt();
    },
    async submit() {
      if (!this.name || this.saving) return;
      this.saving = true;
      try {
        await accessApi.saveGroup(this.name, this.selected);
        eventBus.emit("groupsChanged");
        mutations.closeTopPrompt();
      } catch (e) {
        notify.showError(e);
      } finally {
        this.saving = false;
      }
    },
  },
};
</script>

<style scoped>
.member-list {
  max-height: 16rem;
  overflow-y: auto;
  margin-top: 0.5rem;
}
.member-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.25rem 0;
  cursor: pointer;
}
</style>
