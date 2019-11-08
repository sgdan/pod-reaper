package org.sgdan.podreaper

import io.micronaut.runtime.Micronaut

object Application {

    @JvmStatic
    fun main(args: Array<String>) {
        Micronaut.build()
                .packages("org.sgdan.podreaper")
                .mainClass(Application.javaClass)
                .start()
    }
}
