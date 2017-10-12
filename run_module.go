func RunModule(Name string, Values map[string]interface{}) {
	 /*
	 * Хуки /modules/<module-name>/hooks/before-helm/* в алфавитном порядке
	 * Проверить наличие /modules/<module-name>/Chart.yaml и запустить helm при наличии
	 	* Параметр Values сбросить во временный yaml-файл и подсунуть helm'у
	 * Хуки /modules/<module-name>/hooks/after-helm/* в алфавитном порядке
	 */
}

func RunModuleOnKubeNodeChange() {
	/*
	 *
	 */
}
