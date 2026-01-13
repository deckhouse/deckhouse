workspace "dkp"{

    !identifiers hierarchical

    model {
        // Подключаем свойства
        !include properties.dsl

        // Подключаем архетипы
        !include archetypes.dsl

        // Подключаем акторов
        !include actors.dsl

        // Подключаем внешние системы
        !include externals.dsl

        // Подключаем программную систему
        !include system.dsl

        // Подключаем связи  
        !include relations.dsl

        // Подключаем примеры компонентов схемы для легенды
        !include examples.dsl
    }

    views {
        // Подключаем стили  
        !include styles-fun-v4.dsl

        // Подключаем отображения
        !include views.dsl             
    }

    configuration {
        scope softwaresystem
    }
}