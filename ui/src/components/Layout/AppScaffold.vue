<template>
  <div
    class="max-w-[880px] h-1/2 min-w-0 mx-auto w-full grid grid-cols-[4fr_9fr] bg-mist-800/50 backdrop-blur-sm border border-mist-500 shadow-md"
  >
    <!-- sidebar -->
    <div class="w-full h-full border-r border-mist-500">
      <slot name="bar" />
    </div>
    <!-- router (pages) -->
    <div class="size-full">
      <slot name="page" />
    </div>
  </div>
</template>
