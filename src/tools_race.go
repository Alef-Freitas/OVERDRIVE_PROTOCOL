//go:build tools

// Ferramentas de desenvolvimento (nao entram no binario).
// Mantem o require no go.mod para uso offline (GOPROXY=off).
package main

import (
	_ "github.com/kolkov/racedetector/race"
)
