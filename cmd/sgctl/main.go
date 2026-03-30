package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/PRO-Robotech/cursor/dsl/internal/generator"
	"github.com/PRO-Robotech/cursor/dsl/internal/ir"
	"github.com/PRO-Robotech/cursor/dsl/internal/parser"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "sgctl",
		Short: "SGroups DSL code generator",
	}

	var schemaPath, outputDir, tmplDir string
	var targets []string

	loadAndValidate := func() (*ir.Schema, error) {
		schema, err := parser.ParseFile(schemaPath)
		if err != nil {
			return nil, err
		}
		if errs := parser.Validate(schema); len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  error: %s\n", e)
			}
			return nil, fmt.Errorf("schema validation failed with %d error(s)", len(errs))
		}
		return schema, nil
	}

	genCmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate code from DSL schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, err := loadAndValidate()
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "schema OK: %d resources, %d types, %d roles\n",
				len(schema.Resources), len(schema.Types), len(schema.Roles))

			g := generator.New(schema, outputDir, tmplDir)
			if err := g.Run(targets); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "generation complete")
			return nil
		},
	}
	genCmd.Flags().StringVar(&schemaPath, "schema", "schema.yaml", "Path to DSL schema YAML")
	genCmd.Flags().StringVar(&outputDir, "output", "./generated", "Output directory")
	genCmd.Flags().StringVar(&tmplDir, "templates", "./templates", "Templates directory")
	genCmd.Flags().StringSliceVar(&targets, "target", nil, "Targets to generate (sql, go, proto); empty = all")

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate DSL schema without generating code",
		RunE: func(cmd *cobra.Command, args []string) error {
			schema, err := loadAndValidate()
			if err != nil {
				return err
			}
			fmt.Println("schema is valid")
			names := make([]string, len(schema.Resources))
			for i, r := range schema.Resources {
				names[i] = r.Name
			}
			fmt.Printf("  resources: %s\n", strings.Join(names, ", "))
			fmt.Printf("  types:     %d\n", len(schema.Types))
			fmt.Printf("  roles:     %d\n", len(schema.Roles))
			return nil
		},
	}
	validateCmd.Flags().StringVar(&schemaPath, "schema", "schema.yaml", "Path to DSL schema YAML")

	root.AddCommand(genCmd, validateCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
