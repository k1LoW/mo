# Math Rendering Test

This file tests LaTeX math rendering with KaTeX.

## Inline Math

The quadratic formula is $x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}$.

Einstein's famous equation: $E = mc^2$.

Euler's identity: $e^{i\pi} + 1 = 0$.

## Block Math

The Gaussian integral:

$$
\int_{-\infty}^{\infty} e^{-x^2} dx = \sqrt{\pi}
$$

Sum of first $n$ natural numbers:

$$
\sum_{i=1}^n i = \frac{n(n+1)}{2}
$$

$$F(x, y) = \int_0^{x-y} (x-y-t) f(t)\, dt$$

around text
$$y=x$$
around text

around text $$y=x$$ around text

`$$...$$` inside fenced code blocks must stay untouched:

```bash
echo $$ > /tmp/pid; kill $$
```

## Matrix

A 2x2 matrix:

$$
\begin{pmatrix}
a & b \\
c & d
\end{pmatrix}
$$

## Greek Letters

$\alpha, \beta, \gamma, \delta, \epsilon, \theta, \lambda, \mu, \pi, \sigma, \omega$

## Fractions and Roots

$$
\frac{1}{\sqrt{2}} = \frac{\sqrt{2}}{2}
$$

## Limits and Calculus

$$
\lim_{x \to \infty} \frac{1}{x} = 0
$$

$$
\frac{d}{dx} x^n = nx^{n-1}
$$
