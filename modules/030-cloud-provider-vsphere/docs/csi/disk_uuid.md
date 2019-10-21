Включение поддержки UUID для всех виртуальных машин
=======

Для работы `vsphere-csi-driver` у всех виртуальных машин кластера необходимо включить поддержку параметра `disk.EnableUUID`.

Для этого в интерфейсе vSphere необходимо нажать правой кнопкой на каждую виртуальную машину и выбрать пункт меню: `Edit Settings...` и перейти на вкладку `VM Options`:

![](img/edit_settings.png)

Открыть раздел `Advanced`:

![](img/advanced.png)

И в `Configuration Parameters` нажать на `EDIT CONFIGURATION...`. В данном списке параметров необходимо найти `disk.EnableUUID`, если данного параметра нет, то его необходимо включить. Для этого необходимо:

* Выключить виртуальную машину;
* Перейти в раздел `EDIT CONFIGURATION...` (как было описано выше);
* В правом верхнем угла нажать на кнопку `ADD CONFIGURATION PARAMS`;

![](img/configuration_params.png)

* Ввести имя параметра `disk.EnableUUID` с значением `TRUE`;

![](img/add_new_configuration_params.png)

* Нажать на кнопку `OK`;
* Включить виртуальную машину.
