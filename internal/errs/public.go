package errs

// PublicExitCode normaliza o contrato externo sem perder Code.ExitCode()
func PublicExitCode(err error) int {
	if err == nil {
		return 0
	}
	code := CodeOf(err)
	if code == CodeUsage {
		return 2
	}
	if code == CodeIncomplete {
		return 2
	}
	if code == CodeOK {
		return 0
	}
	return 1
}
