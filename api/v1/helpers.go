package v1

func FindFirstLocation(locations []LocationSpec, locationType LocationType) *LocationSpec {
	for _, location := range locations {
		if location.LocationType == locationType {
			return &location
		}
	}
	return nil
}

func FindAllLocations(locations []LocationSpec, locationType LocationType) []LocationSpec {
	result := make([]LocationSpec, 0)
	for _, location := range locations {
		if location.LocationType == locationType {
			result = append(result, location)
		}
	}
	return result
}
