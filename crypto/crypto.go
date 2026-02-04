package crypto

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"strconv"
	"strings"
)

func encryptString(plainText string) (string, error) {
	//здесь для примера простая base64
	// return base64.StdEncoding.EncodeToString([]byte(plainText)), nil
	return base64.StdEncoding.EncodeToString([]byte(plainText)), nil
}

func EncryptJSONFields(inputJSON []byte, paths []string) ([]byte, error) {
	var root interface{}
	if err := json.Unmarshal(inputJSON, &root); err != nil {
		return nil, err
	}

	// Разделяем paths на позиционные (contain ".") и глобальные имена
	globalNames := map[string]struct{}{}
	positional := [][]string{} // slice of segments

	for _, p := range paths {
		if p == "" {
			continue
		}
		if strings.Contains(p, ".") {
			positional = append(positional, strings.Split(p, "."))
		} else {
			globalNames[p] = struct{}{}
		}
	}

	// Рекурсивный обход: node, positionalPaths (each as []string)
	if err := walkAndEncrypt(root, positional, globalNames); err != nil {
		return nil, err
	}

	out, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// walkAndEncrypt рекурсивно обходит node и:
// - применяет глобальные имена к любым ключам map независимо от глубины
// - пытается применить каждую позиционную path, если она соответствует текущему узлу
func walkAndEncrypt(node interface{}, positional [][]string, globalNames map[string]struct{}) error {
	switch n := node.(type) {
	case map[string]interface{}:
		// Сначала обработаем все ключи: глобальные имена и рекурсивный обход значений
		for k, v := range n {
			// Глобальное имя — если ключ совпадает и значение строка, зашифровать
			if _, ok := globalNames[k]; ok {
				if s, isStr := v.(string); isStr {
					enc, err := encryptString(s)
					if err != nil {
						return err
					}
					n[k] = enc
					// После шифрования значение стало строкой — но не нужно дальше обрабатывать этот узел для глобальных имён
					continue
				}
			}
			// Рекурсивный обход для всех значений (чтобы глобальное имя применилось глубже)
			if err := walkAndEncrypt(v, positional, globalNames); err != nil {
				return err
			}
		}

		// Теперь обработаем позиционные пути: для кажого path, если первый сегмент == some key, спустимся
		for _, segs := range positional {
			if len(segs) == 0 {
				continue
			}
			first := segs[0]
			// если первый сегмент не совпадает с любым ключом — пропускаем
			if val, exists := n[first]; exists {
				if len(segs) == 1 {
					// цель — зашифровать val (если строка)
					if s, isStr := val.(string); isStr {
						enc, err := encryptString(s)
						if err != nil {
							return err
						}
						n[first] = enc
						continue
					} else {
						return fmt.Errorf("value at path %q is not a string", strings.Join(segs, "."))
					}
				}
				// рекурсивно применять оставшиеся сегменты к val
				if err := applyPositionalToNode(val, segs[1:]); err != nil {
					return err
				}
			}
		}
		return nil

	case []interface{}:
		// Рекурсивно обходим элементы массива — глобальные имена внутри элементов будут обработаны
		for i := range n {
			if err := walkAndEncrypt(n[i], positional, globalNames); err != nil {
				return err
			}
		}

		// А также применяем позиционные пути, которые начинаются с индекса/ "*" — но такие пути обрабатываются
		// в applyPositionalToNode при спуске из map.
		return nil

	default:
		// примитивы — ничего не делаем
		return nil
	}
}

// applyPositionalToNode применяется к node согласно сегментам пути, ожидая что текущий вызов
// уже соответствовал некоторому ключу (т.е. мы спустились).
func applyPositionalToNode(node interface{}, segments []string) error {
	if len(segments) == 0 {
		return errors.New("empty positional segments")
	}

	switch n := node.(type) {
	case map[string]interface{}:
		key := segments[0]
		val, exists := n[key]
		if !exists {
			return fmt.Errorf("key %q not found", key)
		}
		if len(segments) == 1 {
			// цель — зашифровать val (если строка)
			if s, ok := val.(string); ok {
				enc, err := encryptString(s)
				if err != nil {
					return err
				}
				n[key] = enc
				return nil
			}
			return fmt.Errorf("value at %q is not a string", strings.Join(segments, "."))
		}
		return applyPositionalToNode(val, segments[1:])

	case []interface{}:
		seg := segments[0]

		// Если сегмент — "*" — применяем путь ко всем элементам массива.
		if seg == "*" {
			for i := range n {
				if len(segments) == 1 {
					// Если это последний сегмент — ожидаем, что элемент массива сам является строкой.
					if s, ok := n[i].(string); ok {
						enc, err := encryptString(s)
						if err != nil {
							return err
						}
						n[i] = enc
					} else {
						return fmt.Errorf("array element %d is not a string", i)
					}
				} else {
					// Иначе рекурсивно спускаемся по оставшимся сегментам.
					if err := applyPositionalToNode(n[i], segments[1:]); err != nil {
						return err
					}
				}
			}
			return nil
		}

		// Попробуем интерпретировать сегмент как индекс массива (число).
		if idx, err := strconv.Atoi(seg); err == nil {
			// Если сегмент — число, применяем к конкретному элементу по индексу.
			if idx < 0 || idx >= len(n) {
				return fmt.Errorf("array index %d out of range", idx)
			}
			if len(segments) == 1 {
				// Последний сегмент — ожидаем строку в элементе по индексу.
				if s, ok := n[idx].(string); ok {
					enc, err := encryptString(s)
					if err != nil {
						return err
					}
					n[idx] = enc
					return nil
				}
				return fmt.Errorf("array element %d is not a string", idx)
			}
			// Рекурсивно спускаемся по оставшимся сегментам в выбранный элемент.
			return applyPositionalToNode(n[idx], segments[1:])
		}

		// Если сегмент не "*" и не число, это означает, что путь вроде "companies.email",
		// где "companies" — массив объектов, а следующий сегмент — ключ внутри каждого элемента.
		// В этом случае применяем те же сегменты к каждому элементу массива (не потребляя сегмент),
		// потому что текущий сегмент относится к полю внутри элементов.
		for i := range n {
			if err := applyPositionalToNode(n[i], segments); err != nil {
				return err
			}
		}
		return nil

	default:
		return fmt.Errorf("unexpected type %T while applying positional path", node)
	}
}
