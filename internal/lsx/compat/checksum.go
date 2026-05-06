package compat

import "lt2_reverse/lsx_server_go/internal/lsxvalue"

func ComputeChecksum(fields map[string]string) int32 {
	date := fieldI32(fields, FieldGameStartingDate)
	revenues := fieldI32(fields, FieldRevenues)
	stands := fieldI32(fields, FieldStands)
	lifespan := fieldI32(fields, FieldLifespan)
	gamegoal := fieldI32(fields, FieldGameGoal)
	standsAssets := fieldI32(fields, FieldStandsAssets)
	cupsSold := fieldI32(fields, FieldCupsSold)
	gamemode := fieldI32(fields, FieldGameMode)
	retained := fieldI32(fields, FieldRetainedEarnings)
	upgrades := fieldI32(fields, FieldUpgradesAssets)
	cash := fieldI32(fields, FieldCashAssets)

	value := date*(revenues-stands*100)*lifespan +
		gamegoal*5 -
		standsAssets -
		cupsSold +
		gamemode*7 +
		retained +
		upgrades +
		cash
	return value
}

func fieldI32(fields map[string]string, name string) int32 {
	return int32(lsxvalue.ParseInt(fields[name]))
}
