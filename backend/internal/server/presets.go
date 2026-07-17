package server

// presetResources define CPU/RAM/disco fixos de cada preset. Preset custom usa
// o Resources vindo do CreateInput sem alteração.
var presetResources = map[Preset]Resources{
	PresetSmall:  {CPUCores: 1, MemoryMB: 1024, DiskGB: 10},
	PresetMedium: {CPUCores: 2, MemoryMB: 4096, DiskGB: 50},
	PresetLarge:  {CPUCores: 4, MemoryMB: 16384, DiskGB: 200},
}

// ResourcesForPreset resolve os recursos efetivos: presets fixos usam a tabela
// acima, custom usa o que o usuário mandou (com defaults mínimos de segurança).
func ResourcesForPreset(preset Preset, custom Resources) Resources {
	if preset == PresetCustom {
		return clampResources(custom)
	}
	if r, ok := presetResources[preset]; ok {
		return r
	}
	return presetResources[PresetSmall]
}

func clampResources(r Resources) Resources {
	if r.CPUCores <= 0 {
		r.CPUCores = 1
	}
	if r.MemoryMB < 256 {
		r.MemoryMB = 256
	}
	if r.DiskGB <= 0 {
		r.DiskGB = 10
	}
	return r
}

// ConfigForResources calcula o subset de postgresql.conf coberto pelo MVP a
// partir da RAM/CPU disponível, seguindo as regras de bolso mais usadas
// (shared_buffers ~25% da RAM, effective_cache_size ~50-75%, etc). Sempre
// editável depois pelo usuário — isso é só o ponto de partida.
func ConfigForResources(r Resources) PostgresConfig {
	sharedBuffers := r.MemoryMB / 4
	if sharedBuffers < 32 {
		sharedBuffers = 32
	}

	effectiveCache := (r.MemoryMB * 3) / 4
	if effectiveCache < sharedBuffers {
		effectiveCache = sharedBuffers
	}

	maintenanceWorkMem := r.MemoryMB / 16
	if maintenanceWorkMem < 16 {
		maintenanceWorkMem = 16
	}
	if maintenanceWorkMem > 2048 {
		maintenanceWorkMem = 2048
	}

	maxConnections := 100
	if r.MemoryMB < 1024 {
		maxConnections = 40
	} else if r.MemoryMB >= 8192 {
		maxConnections = 200
	}

	// work_mem é por operação de ordenação/hash, multiplicado por conexões
	// concorrentes — manter conservador pra não estourar RAM sob carga.
	workMem := (r.MemoryMB / 4) / maxConnections
	if workMem < 2 {
		workMem = 2
	}
	if workMem > 64 {
		workMem = 64
	}

	return PostgresConfig{
		MaxConnections:            maxConnections,
		SharedBuffersMB:           sharedBuffers,
		WorkMemMB:                 workMem,
		MaintenanceWorkMemMB:      maintenanceWorkMem,
		EffectiveCacheSizeMB:      effectiveCache,
		LogMinDurationStatementMs: 1000,
	}
}
