package server

import validator "github.com/zoumo/proxypool/pkg/validator"

type Validators interface {
	Load(name string) (validator.Validator, bool)
	LoadOrStore(name string, in validator.Validator) (validator.Validator, bool)
	Delete(name string)
}

func (s *Server) Validators() Validators {
	return s.validators
}
