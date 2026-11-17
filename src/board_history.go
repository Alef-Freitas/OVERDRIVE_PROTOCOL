//go:build windows

//nolint:wsl_v5 // Historico: nomes curtos seguem a notacao Rust.

package main

import "slices"

// Historico do board em Go puro, derivado de Vec<(SceneCommand, SceneCommand)>
// do Rust: cada passo guarda o comando e seu inverso. operacoes continuas origem
// arraste sao agrupadas por coalesce, como o coalesce_undo do Marble.

type boardCmdKind uint8

const (
	boardCmdMover boardCmdKind = iota
	boardCmdTidy
	boardCmdCriar
	boardCmdExcluir
	boardCmdDobrar
	boardCmdDesdobrar
)

type boardComando struct {
	Kind   boardCmdKind
	IDs    []boardID
	Rects  []boardRect
	Nos    []boardNode
	Pilhas []boardStack
}

type boardParUndo struct {
	Executar boardComando
	Reverter boardComando
}

type boardHistorico struct {
	passos []boardParUndo
	futuro []boardParUndo
}

func (hist *boardHistorico) registrar(executar, reverter boardComando, agrupar bool) {
	if agrupar && len(hist.passos) > 0 {
		ultimo := &hist.passos[len(hist.passos)-1]
		if ultimo.Executar.Kind == executar.Kind && slices.Equal(ultimo.Executar.IDs, executar.IDs) {
			ultimo.Executar = executar

			return
		}
	}

	hist.passos = append(hist.passos, boardParUndo{Executar: executar, Reverter: reverter})
	hist.futuro = nil
}

func (hist *boardHistorico) desfazer(workspace *boardWorkspace) bool {
	if len(hist.passos) == 0 {
		return false
	}

	indice := len(hist.passos) - 1
	par := hist.passos[indice]
	boardAplicarComando(workspace, par.Reverter)
	hist.passos = hist.passos[:indice]
	hist.futuro = append(hist.futuro, par)

	return true
}

func (hist *boardHistorico) refazer(workspace *boardWorkspace) bool {
	if len(hist.futuro) == 0 {
		return false
	}

	indice := len(hist.futuro) - 1
	par := hist.futuro[indice]
	boardAplicarComando(workspace, par.Executar)
	hist.futuro = hist.futuro[:indice]
	hist.passos = append(hist.passos, par)

	return true
}

func (hist *boardHistorico) tamanho() int {
	return len(hist.passos)
}

func boardAplicarComando(workspace *boardWorkspace, cmd boardComando) {
	switch cmd.Kind {
	case boardCmdMover, boardCmdTidy:
		boardAplicarRects(workspace, cmd.IDs, cmd.Rects)
	case boardCmdCriar:
		workspace.Nodes = append(workspace.Nodes, cmd.Nos...)
	case boardCmdExcluir:
		for _, identificador := range cmd.IDs {
			boardRemoverNo(workspace, identificador)
		}
	case boardCmdDobrar:
		workspace.Stacks = append(workspace.Stacks, cmd.Pilhas...)
	case boardCmdDesdobrar:
		workspace.Stacks = slices.DeleteFunc(workspace.Stacks, func(pilha boardStack) bool {
			return slices.Contains(cmd.IDs, pilha.Front)
		})
	}
}

func boardAplicarRects(workspace *boardWorkspace, ids []boardID, rects []boardRect) {
	for indice, identificador := range ids {
		if indice >= len(rects) {
			break
		}
		if noAtual := boardNo(workspace, identificador); noAtual != nil {
			noAtual.Rect = rects[indice]
		}
	}
}

func boardComandoMover(ids []boardID, antes, depois []boardRect) (boardComando, boardComando) {
	return boardComando{
		Kind: boardCmdMover, IDs: ids, Rects: depois, Nos: nil, Pilhas: nil,
	}, boardComando{
		Kind: boardCmdMover, IDs: ids, Rects: antes, Nos: nil, Pilhas: nil,
	}
}

func boardComandoTidy(ids []boardID, antes, depois []boardRect) (boardComando, boardComando) {
	return boardComando{
		Kind: boardCmdTidy, IDs: ids, Rects: depois, Nos: nil, Pilhas: nil,
	}, boardComando{
		Kind: boardCmdTidy, IDs: ids, Rects: antes, Nos: nil, Pilhas: nil,
	}
}
