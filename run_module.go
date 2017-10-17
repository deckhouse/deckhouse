package main

func RunModule(Name string, Values map[string]interface{}) error {
	/*
	 * Хуки /modules/<module-name>/hooks/before-helm/* в алфавитном порядке
	 * Проверить наличие /modules/<module-name>/Chart.yaml и запустить helm при наличии
	 	* Параметр Values сбросить во временный yaml-файл и подсунуть helm'у
	 * Хуки /modules/<module-name>/hooks/after-helm/* в алфавитном порядке
	*/
	return nil
}

func RunModuleOnKubeNodeChange() error {
	return nil
}
