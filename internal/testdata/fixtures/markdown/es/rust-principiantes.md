---
title: "Rust para Principiantes"
lang: es
---

# Rust para Principiantes

Rust es un lenguaje de programación de sistemas que se enfoca en seguridad, concurrencia y rendimiento.

## Propiedad (Ownership)

La propiedad es la característica más única de Rust:

1. Cada valor tiene una variable que es su *propietaria*.
2. Solo puede haber un propietario a la vez.
3. Cuando el propietario sale del alcance, el valor se libera.

## Ejemplo Básico

```rust
fn main() {
    let s = String::from("hola");
    toma_propiedad(s);
    // println!("{}", s); // ERROR: s ya no es válido
}

fn toma_propiedad(algun_string: String) {
    println!("{}", algun_string);
}
```

## Préstamos

Las referencias permiten prestar sin tomar propiedad:

```rust
fn calcular_longitud(s: &String) -> usize {
    s.len()
}
```

## Seguridad de Memoria

El compilador de Rust previene:
- Punteros colgantes
- Liberaciones dobles
- Condiciones de carrera
