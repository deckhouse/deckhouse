<template>
  <header class="sticky top-0 inset-x-0 flex flex-wrap z-[48] w-full bg-white border-b text-sm py-5 pl-64">
    <nav class="flex basis-full items-center w-full mx-auto px-10" aria-label="Global">
      <div class="w-full flex items-center justify-end ml-auto justify-between gap-x-3 order-3">
        <ol class="flex items-center whitespace-nowrap min-w-0" aria-label="Breadcrumb">
          <li class="text-sm text-gray-500">
            <a class="flex items-center hover:text-blue-600" href="#">
              Кластер
              <svg
                class="flex-shrink-0 h-5 w-5 text-gray-400 mx-2"
                width="16"
                height="16"
                viewBox="0 0 16 16"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
                aria-hidden="true"
              >
                <path d="M6 13L10 3" stroke="currentColor" stroke-linecap="round" />
              </svg>
            </a>
          </li>

          <li class="text-sm font-semibold text-black truncate" aria-current="page">Обновления</li>
        </ol>
      </div>
    </nav>
  </header>
</template>
