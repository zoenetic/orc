package orc

import "context"

type PkgSource interface {
	Name() string
	Available() bool
	Install(ctx context.Context, p *Pkg) error
}

func Apt() PkgSource    { return &aptSource{} }
func Dnf() PkgSource    { return &dnfSource{} }
func Nix() PkgSource    { return &nixSource{} }
func Winget() PkgSource { return &wingetSource{} }

type Pkg struct {
	name    string
	sources []PkgSource
}

type aptSource struct{}

func (s aptSource) Name() string    { return "apt" }
func (s aptSource) Available() bool { return true }
func (s aptSource) Install(ctx context.Context, p *Pkg) error {
	return nil
}

type dnfSource struct{}

func (s dnfSource) Name() string    { return "dnf" }
func (s dnfSource) Available() bool { return true }
func (s dnfSource) Install(ctx context.Context, p *Pkg) error {
	return nil
}

type nixSource struct{}

func (s nixSource) Name() string    { return "nix" }
func (s nixSource) Available() bool { return true }
func (s nixSource) Install(ctx context.Context, p *Pkg) error {
	return nil
}

type wingetSource struct{}

func (s wingetSource) Name() string    { return "winget" }
func (s wingetSource) Available() bool { return true }
func (s wingetSource) Install(ctx context.Context, p *Pkg) error {
	return nil
}
