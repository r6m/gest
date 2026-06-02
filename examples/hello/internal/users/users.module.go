package users

import "github.com/r6m/gest"

// Module wires the users feature providers and controller.
func Module() gest.Module {
	return gest.NewModule(gest.ModuleConfig{
		Name: "UsersModule",
		Providers: gest.Providers(
			gest.Provide(NewUserService),
			gest.Controller(NewUserController),
		),
	})
}
