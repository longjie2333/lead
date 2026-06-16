<template>
  <section class="flex min-h-0 flex-1 items-center justify-center p-4">
    <div class="w-full max-w-md rounded-2xl bg-white p-6 text-center shadow-soft">
      <MonitorSmartphone class="mx-auto h-10 w-10 text-primary" />
      <h2 class="mt-4 text-lg font-semibold text-ink">选择设备开始远程协助</h2>
      <p class="mt-2 text-sm text-gray-600">大屏幕下设备列表会保持在左侧，远控画面会显示在右侧。</p>
    </div>
  </section>
</template>

<script setup>
import { MonitorSmartphone } from 'lucide-vue-next'
</script>
