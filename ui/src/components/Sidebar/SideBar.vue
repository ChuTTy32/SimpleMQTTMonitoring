<template>
  <!-- minmax(0,1fr) явно говорит браузеру: минимальная высота 0, максимальная 1fr -->
  <div class="grid grid-rows-[4rem_minmax(0,1fr)_4rem] h-full overflow-hidden">
    <div class="bg-blue-400">user block</div>

    <div class="bg-green-400 min-h-0">
      <div class="h-12 shrink-0">menu search</div>

      <div id="menu-list" class="bg-yellow-400 overflow-y-auto">
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
        <div>element</div>
      </div>
    </div>

    <div class="bg-red-400">settings button</div>
  </div>
</template>
