<script setup lang="ts">
import { RouterView } from 'vue-router'
import AppScaffold from './components/Layout/AppScaffold.vue'
import SideBar from './components/Sidebar/SideBar.vue'
</script>

<template>
  <div class="bg-mist-900 grid items-center justify-stretch min-h-dvh w-full p-4">
    <AppScaffold>
      <template #bar>
        <SideBar />
      </template>
      <template #page>
        <RouterView />
      </template>
    </AppScaffold>
  </div>
</template>
