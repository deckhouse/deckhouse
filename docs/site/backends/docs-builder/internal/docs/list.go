package docs

type Version struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

func (svc *Service) List() (versions []Version, err error) {
	cm := svc.channelMappingEditor.get()

	for moduleName := range cm {
		for channels := range cm[moduleName] {
			for _, entity := range cm[moduleName][channels] {
				versions = append(versions, Version{
					Module:  moduleName,
					Version: entity.Version,
				})
			}
		}
	}

	return versions, nil
}
