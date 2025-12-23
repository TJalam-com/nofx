import { motion } from 'framer-motion'
import { useState, useEffect } from 'react'
import { NeuronPulse } from './NeuronPulse'

interface NeuralNetworkVizProps {
  isActive: boolean
  layerCount?: number
  nodesPerLayer?: number[]
  className?: string
}

/**
 * Visual representation of neural network layers.
 * Nodes light up in sequence (input → hidden → output layers).
 * Color pulses propagate through connections.
 */
export function NeuralNetworkViz({
  isActive,
  layerCount = 3,
  nodesPerLayer = [4, 6, 2],
  className = '',
}: NeuralNetworkVizProps) {
  const [activeLayer, setActiveLayer] = useState<number>(-1)
  const [activeNodes, setActiveNodes] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (!isActive) {
      setActiveLayer(-1)
      setActiveNodes(new Set())
      return
    }

    // Sequential layer activation
    const layerInterval = setInterval(() => {
      setActiveLayer((prev) => {
        const next = (prev + 1) % layerCount
        return next
      })
    }, 2000)

    // Node activation within layers
    const nodeInterval = setInterval(() => {
      setActiveNodes((prev) => {
        const newSet = new Set(prev)
        const currentLayer = activeLayer >= 0 ? activeLayer : 0
        const nodeCount = nodesPerLayer[currentLayer] || 4

        // Activate random nodes in current layer
        for (let i = 0; i < Math.min(2, nodeCount); i++) {
          const nodeId = `${currentLayer}-${Math.floor(Math.random() * nodeCount)}`
          newSet.add(nodeId)
        }

        // Remove old activations
        if (newSet.size > 10) {
          const toRemove = Array.from(newSet).slice(0, 5)
          toRemove.forEach((id) => newSet.delete(id))
        }

        return newSet
      })
    }, 300)

    return () => {
      clearInterval(layerInterval)
      clearInterval(nodeInterval)
    }
  }, [isActive, layerCount, nodesPerLayer, activeLayer])

  const getNodeColor = (layerIndex: number): string => {
    if (layerIndex === 0) return '#60a5fa' // Input layer - blue
    if (layerIndex === layerCount - 1) return '#0ECB81' // Output layer - green
    return '#c084fc' // Hidden layers - purple
  }

  const getConnectionColor = (fromLayer: number, toLayer: number): string => {
    if (fromLayer < toLayer && activeLayer >= fromLayer && activeLayer <= toLayer) {
      return getNodeColor(fromLayer)
    }
    return 'rgba(255, 255, 255, 0.1)'
  }

  return (
    <div
      className={className}
      style={{
        padding: '20px',
        background: 'var(--navy-dark)',
        borderRadius: '8px',
        border: '1px solid var(--panel-border)',
        minHeight: '200px',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-around',
          alignItems: 'center',
          height: '100%',
          position: 'relative',
        }}
      >
        {Array.from({ length: layerCount }).map((_, layerIndex) => (
          <div
            key={layerIndex}
            style={{
              display: 'flex',
              flexDirection: 'column',
              gap: '12px',
              alignItems: 'center',
            }}
          >
            {/* Layer label */}
            <div
              style={{
                fontSize: '10px',
                color: '#848E9C',
                marginBottom: '8px',
                textTransform: 'uppercase',
                fontWeight: 'bold',
              }}
            >
              {layerIndex === 0
                ? 'Input'
                : layerIndex === layerCount - 1
                  ? 'Output'
                  : `Hidden ${layerIndex}`}
            </div>

            {/* Nodes */}
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: '12px',
              }}
            >
              {Array.from({ length: nodesPerLayer[layerIndex] || 4 }).map(
                (_, nodeIndex) => {
                  const nodeId = `${layerIndex}-${nodeIndex}`
                  const isNodeActive =
                    activeNodes.has(nodeId) && activeLayer === layerIndex

                  return (
                    <NeuronPulse
                      key={nodeId}
                      isActive={isActive && isNodeActive}
                      delay={nodeIndex * 100}
                      color={getNodeColor(layerIndex)}
                      size={12}
                    />
                  )
                }
              )}
            </div>
          </div>
        ))}

        {/* Connections between layers */}
        {layerCount > 1 &&
          Array.from({ length: layerCount - 1 }).map((_, layerIndex) => (
            <motion.div
              key={`connection-${layerIndex}`}
              style={{
                position: 'absolute',
                left: `${(layerIndex + 0.5) * (100 / layerCount)}%`,
                top: '50%',
                width: `${100 / layerCount}%`,
                height: '2px',
                background: getConnectionColor(layerIndex, layerIndex + 1),
                transform: 'translateY(-50%)',
                zIndex: 0,
              }}
              animate={{
                opacity: [0.3, 0.8, 0.3],
              }}
              transition={{
                duration: 2,
                repeat: Infinity,
                ease: 'easeInOut',
              }}
            />
          ))}
      </div>

      {/* Subtle background gradient animation */}
      {isActive && (
        <motion.div
          style={{
            position: 'absolute',
            top: 0,
            left: 0,
            right: 0,
            bottom: 0,
            background:
              'radial-gradient(circle at 50% 50%, rgba(96, 165, 250, 0.05) 0%, transparent 70%)',
            pointerEvents: 'none',
          }}
          animate={{
            opacity: [0.3, 0.6, 0.3],
          }}
          transition={{
            duration: 3,
            repeat: Infinity,
            ease: 'easeInOut',
          }}
        />
      )}
    </div>
  )
}
