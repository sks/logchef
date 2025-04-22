<script setup lang="ts">
// Layout component for logs section
</script>

<template>
    <router-view v-slot="{ Component }">
        <keep-alive>
            <component :is="Component" />
        </keep-alive>
    </router-view>
</template>